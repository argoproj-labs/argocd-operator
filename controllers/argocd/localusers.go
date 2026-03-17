// Copyright 2025 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"context"
	json "encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	"github.com/argoproj/argo-cd/v3/util/settings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// Constant used as part of the subject for the JWT
	localUserApiKey = "apiKey"

	// Value to set for the "app.kubernetes.io/component" label on the user
	// secret
	localUserSecretComponent = "local-users"
)

// These are the constants we set in .data field of Secret "(user name)-local-user"
const (

	// Key for the JWT in the user secret's data
	localUserApiToken = "apiToken"

	// Key used to store the current autorenew value in the user secret so we
	// can tell if it has been changed in the ArgoCD CR
	localUserAutoRenew = "autoRenew"

	// Key used to store when the token expires in the user secret
	localUserExpiresAt = "expAt"

	// Key used to store the current token lifetime value in the user secret so
	// we can tell if it has been changed in the ArgoCD CR
	localUserTokenLifetime = "tokenLifetime"

	// Key to store the user name in the user secret
	localUserUser = "user"
)

func (r *ReconcileArgoCD) reconcileLocalUsers(cr argoproj.ArgoCD) error {

	ctx := context.TODO()

	// Since it's possible for multiple go-routines to call this function simultaneously, we hold a blanket lock here for any local user reconciliation
	r.LocalUsers.lock.Lock()
	defer r.LocalUsers.lock.Unlock()

	// Retrieve latest copy of ArgoCD CR to ensure we are working on recent (non-stale) data, rather than using 'cr' param as is
	if err := r.Get(ctx, client.ObjectKeyFromObject(&cr), &cr); err != nil {

		if apierrors.IsNotFound(err) {
			// CR does not exist, so no more work to do
			return nil
		}

		return fmt.Errorf("unable to retrieve ArgoCD CR in reconcileLocalUsers: %v", err)
	}

	// Reconcile the local users declared in the argocd CR
	legacyUsers := localUsersInExtraConfig(cr)
	for _, user := range cr.Spec.LocalUsers {
		if legacyUsers[user.Name] {
			log.Info("Skipping local user reconciliation, user is defined in extraConfig", "localUserName", user.Name)
			continue
		}
		if err := r.reconcileLocalUser(ctx, cr, user); err != nil {
			return err
		}
	}

	// Delete the secret and token for users that are no longer in the
	// localUsers section of the argocd CR
	if err := r.cleanupLocalUsers(ctx, cr); err != nil {
		return err
	}

	return nil
}

// getArgoCDSigningKey retrieve the signing key from the argocd-secret
func (r *ReconcileArgoCD) getArgoCDSigningKey(cr argoproj.ArgoCD) ([]byte, error) {
	argoCDSecret := corev1.Secret{}
	found, err := argoutil.IsObjectFound(r.Client, cr.Namespace, common.ArgoCDSecretName, &argoCDSecret)
	if err != nil {
		return nil, fmt.Errorf("error retrieving secret %s: %w", common.ArgoCDSecretName, err)
	}
	if !found {
		return nil, fmt.Errorf("could not find secret %s", common.ArgoCDSecretName)
	}
	signingKey, ok := argoCDSecret.Data["server.secretkey"]
	if !ok {
		return nil, fmt.Errorf("could not find server.secretkey in secret %s", argoCDSecret.Name)
	}
	return signingKey, nil

}

// stopLocalUserTimer stops timer used to track token renewal (but only if the timer is present, otherwise no-op)
// - LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) stopLocalUserTimer(cr argoproj.ArgoCD, userName string, reason string) {

	// Locate and stop existing timer
	key := tokenRenewalTimerKeyLocalUser(cr.Namespace, userName)
	existingTimer, ok := r.LocalUsers.tokenRenewalTimers[key]
	if ok {
		if reason != "" {
			log.Info(fmt.Sprintf("token renewal timer was stopped for '%s': %s", userName, reason))
		} else {
			log.Info(fmt.Sprintf("token renewal timer was stopped for '%s'", userName))
		}

		existingTimer.stopped = true
		existingTimer.timer.Stop()
		delete(r.LocalUsers.tokenRenewalTimers, key)
	}
}

// reconcileLocalUser verifies that a token/secret for a specific user is up to date
// - LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) reconcileLocalUser(ctx context.Context, cr argoproj.ArgoCD, user argoproj.LocalUserSpec) error {

	log := log.WithValues("localUserName", user.Name)

	user.TokenLifetime = strings.TrimSpace(user.TokenLifetime)

	// Ensure the timer is stopped for all cases where it should be stopped
	{
		var reasonToStopTimer string

		if user.Enabled != nil && !*user.Enabled {
			reasonToStopTimer = "user is not enabled"
		}
		if user.AutoRenewToken != nil && !*user.AutoRenewToken {
			reasonToStopTimer = "token auto renew is false"
		}

		if user.TokenLifetime == "" || user.TokenLifetime == "0" {
			reasonToStopTimer = "token lifetime is set not to expire: infinite lifetime"
		}
		if user.ApiKey != nil && !*user.ApiKey {
			reasonToStopTimer = "apiKey field of user is false"
		}

		if reasonToStopTimer != "" {
			r.stopLocalUserTimer(cr, user.Name, reasonToStopTimer)
		}
	}

	var tokenDuration time.Duration
	tokenLifetime := user.TokenLifetime
	autoRenew := "true"
	{
		if tokenLifetime != "" {
			val, err := time.ParseDuration(user.TokenLifetime)
			if err != nil {
				return fmt.Errorf("failed to parse token lifetime for user %s: %w", user.Name, err)
			}
			if val < 0 {
				return fmt.Errorf("token lifetime for user '%s' must not be negative: '%s'", user.Name, user.TokenLifetime)
			}
			tokenDuration = val
		}

		if tokenLifetime == "" || tokenDuration == 0 {
			tokenLifetime = "0h"
		}

		if user.AutoRenewToken != nil && !*user.AutoRenewToken {
			autoRenew = "false"
		}
	}

	// Get the '(username)-local-user' user secret if it exists
	localUserSecret := corev1.Secret{}

	// apikeyIsFalseOrEnabledIsFalse is whether the local secret and argocd token should be deleted
	apikeyIsFalseOrEnabledIsFalse := (user.ApiKey != nil && !*user.ApiKey) || (user.Enabled != nil && !*user.Enabled)

	if err := r.Get(ctx, types.NamespacedName{Name: secretNameLocalUser(user), Namespace: cr.Namespace}, &localUserSecret); err != nil {

		if apierrors.IsNotFound(err) {

			if apikeyIsFalseOrEnabledIsFalse {
				// Secret doesn't exist, and shouldn't exist, so just clean up argo cd secret, then return

				// We don't need to stop any renewal timers, as this is already handled at the top of the function
				return r.cleanupLocalUser(ctx, cr, user.Name, nil)
			}

			// Secret doesn't exist, so create it and schedule renewal if required

			log.Info("Local user secret doesn't exist, so creating it")

			err = r.issueToken(ctx, cr, user, tokenLifetime, tokenDuration)

			if err == nil {
				if tokenDuration > 0 && autoRenew == "true" {
					r.startOrRestartLocalUserTimer(cr, user.Name, tokenDuration)
				} else {
					log.Info("Not scheduling renewal for secret, as it is not needed")
				}
			}
			return err
		}

		// Otherwise for any other generic error
		return fmt.Errorf("failed to get user secret for user %s: %w", user.Name, err)
	}

	// From this point on in the function, we can assume secret exists

	// If the ApiKey is false, or user is not enabled, then delete the secret/token, and return
	if apikeyIsFalseOrEnabledIsFalse {
		log.Info("Deleting local user secret and token because user is disabled or apiKey is false")
		// We don't need to stop any renewal timers, as this is already handled at the top of the function
		return r.cleanupLocalUser(ctx, cr, user.Name, &localUserSecret)
	}

	// From this point on we can assume that apiKey is true and enabled is true

	expAt, err := strconv.ParseInt(string(localUserSecret.Data[localUserExpiresAt]), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert \"%s\" value to int64 from user secret %s: %w", localUserExpiresAt, localUserSecret.Name, err)
	}

	// The secret exists at this point, so the questions are:
	// - Has the user changed any of the ArgoCD CR values such that the secret is no longer consistent
	// - Is the secret expired
	// - Is the 'common.ArgoCDSecretName' Secret out of sync?

	// deleteAndCreateNewToken is true if we need to replace token (for example, the token has expired (and autoRenew is true), or the token lifetime changed, or the current token is invalid), false otherwise
	deleteAndCreateNewToken := false

	if tokenLifetime != string(localUserSecret.Data[localUserTokenLifetime]) {
		// If the user changed the lifetime of the token in the ArgoCD CR, then we need to invalidate and recreate the token
		deleteAndCreateNewToken = true
		log.Info("Create new local user token as 'tokenLifetime' value has changed. Old value: '" + string(localUserSecret.Data[localUserTokenLifetime]) + "', new value:'" + tokenLifetime + "'")
	}

	if expAt == 0 && tokenDuration != 0 {
		// If expAt == 0 value in the field is invalid, we need to recreate the token
		deleteAndCreateNewToken = true
		log.Error(nil, "The expAt value in the local user Secret is invalid, recreating local user token/secret")
	}

	if autoRenew == "true" && expAt != 0 {
		renewalTime := time.Unix(expAt, 0)
		if time.Now().After(renewalTime) {
			// If the current token has expired, and auto-renew is true, then renew it
			log.Info("The local user token has expired and will be renewed. ExpirationTime was: '" + string(localUserSecret.Data[localUserExpiresAt]) + "'")
			deleteAndCreateNewToken = true
		}
	}

	if !deleteAndCreateNewToken {
		argoCDSecret := corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret)
		if err != nil {
			return fmt.Errorf("unable to retrieve Argo CD Secret '%s': %v", common.ArgoCDSecretName, err)
		}

		//  If Secret common.ArgoCDSecretName is missing our token entry, then we should regenerate it
		key := fmt.Sprintf("accounts.%s.tokens", user.Name)
		if _, exists := argoCDSecret.Data[key]; !exists {
			deleteAndCreateNewToken = true
		}
	}

	if deleteAndCreateNewToken {

		// Issue new token, create/update user secret, update argo cd secret

		if err := r.issueToken(ctx, cr, user, tokenLifetime, tokenDuration); err != nil {
			return fmt.Errorf("unable to issue new token for user '%s': %v", user.Name, err)
		}

		// Since a new token was issued, we need to reschedule the renewal (if applicable)
		if tokenDuration > 0 && autoRenew == "true" {
			r.startOrRestartLocalUserTimer(cr, user.Name, tokenDuration)
		}

		return nil

	}

	// From this point on, we've verified the token does not need to be regenerated.

	// If user has changed autoRenewToken field in ArgoCD CR (versus the value in local user secret), then we don't need to recreate the local user secret, BUT we must update that secret with new 'autoRenewToken' field value
	if string(localUserSecret.Data[localUserAutoRenew]) != autoRenew {

		// Secret is not up-to-date with auto-renew field, so update it

		localUserSecret.Data[localUserAutoRenew] = ([]byte)(autoRenew)
		argoutil.LogResourceUpdate(log, &localUserSecret, "autoRenew set to "+autoRenew+" for user", tokenRenewalTimerKeyLocalUser(cr.Namespace, user.Name))
		err = r.Update(ctx, &localUserSecret)

		if err != nil {
			return fmt.Errorf("unable to update local user secret for user '%s': %v", user.Name, err)
		}

		// if autoRenew went from true -> false, the renewal was already unscheduled at the top of this function, so no work to do
	}

	// Finally, verify the token renewal is scheduled (if applicable)
	if autoRenew == "true" && tokenDuration != 0 {

		// Schedule renewal if it isn't already scheduled
		key := tokenRenewalTimerKeyLocalUser(cr.Namespace, user.Name)
		_, ok := r.LocalUsers.tokenRenewalTimers[key]
		if !ok {

			remainingTime := tokenDuration

			if expAt != 0 {
				remainingTime = time.Until(time.Unix(expAt, 0))
			}

			log.Info("Scheduling local user token renewal, as it was not previously scheduled")
			r.startOrRestartLocalUserTimer(cr, user.Name, remainingTime)
		}

	}

	return nil
}

// LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) issueToken(ctx context.Context, cr argoproj.ArgoCD, user argoproj.LocalUserSpec, tokenLifetime string, tokenDuration time.Duration) error {

	log.Info("Issuing token for local user", "localUserName", user.Name, "tokenLifetime", tokenLifetime)

	// Get the user secret if it exists, else create a new one
	userSecret := corev1.Secret{}
	secretExists := true
	{
		secretName := secretNameLocalUser(user)
		err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: cr.Namespace}, &userSecret)
		if apierrors.IsNotFound(err) {
			secretExists = false
			userSecret = *argoutil.NewSecretWithName(&cr, secretName)
			userSecret.Labels[common.ArgoCDKeyComponent] = localUserSecretComponent
			if err := controllerutil.SetControllerReference(&cr, &userSecret, r.Scheme); err != nil {
				return err
			}
		} else if err != nil {
			return fmt.Errorf("failed to get user secret for user %s: %w", user.Name, err)
		}
	}

	// Create the values from which the token is generated

	var uniqueId string
	if u, err := uuid.NewRandom(); err != nil {
		return fmt.Errorf("failed to generate unique ID for token for user %s: %w", user.Name, err)
	} else {
		uniqueId = u.String()
	}

	issuedAt := time.Now()

	var expiresAt int64
	if tokenDuration > 0 {
		expiresAt = issuedAt.Add(tokenDuration).Unix()
	}

	signingKey, err := r.getArgoCDSigningKey(cr)
	if err != nil {
		return fmt.Errorf("unable to retrieve Argo CD signing key when attempting to issue token for user %s: %v", user.Name, err)
	}

	subject := fmt.Sprintf("%s:%s", user.Name, localUserApiKey)
	jwtToken, err := createJWTToken(subject, issuedAt, tokenDuration, uniqueId, signingKey)
	if err != nil {
		return fmt.Errorf("error creating token for user %s: %w", user.Name, err)
	}

	// We store the the values of TokenLifetime and AutoRenew in the user secret
	// so we can tell if they've been changed in the ArgoCD CR

	var autoRenew string
	if user.AutoRenewToken == nil || *user.AutoRenewToken {
		autoRenew = "true"
	} else {
		autoRenew = "false"
	}
	userSecret.Data = map[string][]byte{
		localUserUser:          []byte(user.Name),
		localUserExpiresAt:     []byte(strconv.FormatInt(expiresAt, 10)),
		localUserTokenLifetime: []byte(tokenLifetime),
		localUserAutoRenew:     []byte(autoRenew),
		localUserApiToken:      []byte(jwtToken),
	}

	// Create or update the local user secret
	if secretExists {
		argoutil.LogResourceUpdate(log, &userSecret)
		if err := r.Update(ctx, &userSecret); err != nil {
			return err
		}
	} else {
		argoutil.LogResourceCreation(log, &userSecret)
		if err := r.Create(ctx, &userSecret); err != nil {
			return err
		}
	}

	// Add the token info to the argocd-secret

	accountTokens := []settings.Token{
		{
			ID:        uniqueId,
			IssuedAt:  issuedAt.Unix(),
			ExpiresAt: expiresAt,
		},
	}

	tokens, err := json.Marshal(accountTokens)
	if err != nil {
		return err
	}

	argoCDSecret := corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("accounts.%s.tokens", user.Name)
	argoCDSecret.Data[key] = tokens
	argoutil.LogResourceUpdate(log, &argoCDSecret, "setting token for user account", user.Name)
	if err := r.Update(ctx, &argoCDSecret); err != nil {
		return err
	}

	return nil
}

// startOrRestartLocalUserTimer starts (or restarts, if already present) the local user timer for a particular local user
// - LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) startOrRestartLocalUserTimer(cr argoproj.ArgoCD, userName string, tokenDuration time.Duration) {

	// Locate and stop existing timer (if it exists)
	r.stopLocalUserTimer(cr, userName, "")

	// Create and start new timer
	renewalTimer := &tokenRenewalTimer{}
	timer := time.AfterFunc(tokenDuration, func() {

		r.LocalUsers.lock.Lock()
		defer r.LocalUsers.lock.Unlock()

		if renewalTimer.stopped {
			return
		}

		// On timeout, reconcile all local users on the ArgoCD CR

		go func() {
			log.Info("Timer elapsed, triggering renewal of local user token: '" + userName + "' in " + cr.Namespace)

			if err := r.reconcileLocalUsers(cr); err != nil {
				log.Error(err, "Error occurred on renewal of local user token, the renewal will be attempted again on next ArgoCD CR reconciliation.")
			}
		}()

	})
	renewalTimer.timer = timer
	key := tokenRenewalTimerKeyLocalUser(cr.Namespace, userName)
	r.LocalUsers.tokenRenewalTimers[key] = renewalTimer
	msg := fmt.Sprintf("Scheduled token renewal for user '%s' to %s", key, time.Now().Add(tokenDuration).Format(time.RFC1123))
	log.Info(msg)

}

// cleanupLocalUsers looks at local user secrets in a namespace, and cleans up any that are not in the ArgoCD CR (which is the source of truth for which local users should exist)
// - LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) cleanupLocalUsers(ctx context.Context, cr argoproj.ArgoCD) error {
	// Get a list of the local user secrets
	secrets := corev1.SecretList{}
	options := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDKeyComponent: localUserSecretComponent,
		}),
		Namespace: cr.Namespace,
	}
	if err := r.List(ctx, &secrets, &options); err != nil {
		return err
	}

	// Clean up the local user secrets and argocd-secret tokens for any users
	// that aren't declared in the localUsers section of the argocd CR
	for idx := range secrets.Items {
		localUserSecret := secrets.Items[idx]

		userName := string(localUserSecret.Data[localUserUser])
		found := false
		for _, user := range cr.Spec.LocalUsers {
			if userName == user.Name {
				found = true
				break
			}
		}
		if !found {
			r.stopLocalUserTimer(cr, userName, "local user no longer defined in ArgoCD CR")
			if err := r.cleanupLocalUser(ctx, cr, userName, &localUserSecret); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanupLocalUser deleted the local user secret, and removes the token from argocd-secret Secret
// - LocalUsers lock should be owned when calling this
func (r *ReconcileArgoCD) cleanupLocalUser(ctx context.Context, cr argoproj.ArgoCD, userName string, localUserSecret *corev1.Secret) error {

	if localUserSecret != nil {
		if err := r.Delete(ctx, localUserSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			// No Secret to delete, so continue...
		} else {
			argoutil.LogResourceDeletion(log, localUserSecret, "deleted secret for local user secret for user", userName)
		}
	}

	// Don't delete the token from the argocd-secret if the user is in extraConfig and using an apiKey
	if strings.Contains(cr.Spec.ExtraConfig["accounts."+userName], localUserApiKey) {
		log.Info("Not removing token from argocd-secret, user is defined in extraConfig with apiKey", "localUserName", userName)
		return nil
	}

	argoCDSecret := corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret); err != nil {
		return err
	}

	key := fmt.Sprintf("accounts.%s.tokens", userName)
	if _, ok := argoCDSecret.Data[key]; ok {
		argoutil.LogResourceUpdate(log, &argoCDSecret, "deleting token for local user", userName)
		delete(argoCDSecret.Data, key)
		if err := r.Update(ctx, &argoCDSecret); err != nil {
			return err
		}
	}

	return nil
}

func createJWTToken(subject string, issuedAt time.Time, expiresIn time.Duration, id string, serverSignature []byte) (string, error) {
	issuedAt = issuedAt.UTC()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		Issuer:    "argocd",
		NotBefore: jwt.NewNumericDate(issuedAt),
		Subject:   subject,
		ID:        id,
	}
	if expiresIn > 0 {
		expires := issuedAt.Add(expiresIn)
		claims.ExpiresAt = jwt.NewNumericDate(expires)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(serverSignature)
}

// localUsersInExtraConfig returns the names of all users explicitly defined via
// extraConfig
func localUsersInExtraConfig(cr argoproj.ArgoCD) map[string]bool {
	localUsers := make(map[string]bool)
	for k := range cr.Spec.ExtraConfig {
		if strings.HasPrefix(k, "accounts.") && !strings.HasSuffix(k, ".enabled") {
			user := k[len("accounts."):]
			if !strings.Contains(user, ".") {
				localUsers[user] = true
			} else {
				log.Info("ignoring invalid local user account name in extra config", "key:", k)
			}
		}
	}
	return localUsers
}

func (r *ReconcileArgoCD) cleanupNamespaceTokenTimers(namespace string) {

	r.LocalUsers.lock.Lock()
	defer r.LocalUsers.lock.Unlock()

	log.Info(fmt.Sprintf("removing local user token renewal timers for namespace \"%s\"", namespace))
	prefix := namespace + "/"
	for key, timer := range r.LocalUsers.tokenRenewalTimers {
		if strings.HasPrefix(key, prefix) {
			timer.stopped = true
			timer.timer.Stop()
			delete(r.LocalUsers.tokenRenewalTimers, key)
		}
	}
}

func secretNameLocalUser(user argoproj.LocalUserSpec) string {
	return user.Name + "-local-user"
}

func tokenRenewalTimerKeyLocalUser(namespace string, name string) string {
	return namespace + "/" + name
}
