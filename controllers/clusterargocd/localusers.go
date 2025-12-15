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

package clusterargocd

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

	// Key for the JWT in the user secret's data
	localUserApiToken = "apiToken"

	// Key used to store the current autorenew value in the user secret so we
	// can tell if it has been changed in the ArgoCD CR
	localUserAutoRenew = "autoRenew"

	// Key used to store when the token expires in the user secret
	localUserExpiresAt = "expAt"

	// Key used to store the current token lifetim value in the user secret so
	// we can tell if it has been changed in the ArgoCD CR
	localUserTokenLifetime = "tokenLifetime"

	// Key to store the user name in the user secret
	localUserUser = "user"

	// Value to set for the "app.kubernetes.io/component" label on the user
	// secret
	localUserSecretComponent = "local-users"
)

func (r *ReconcileClusterArgoCD) reconcileLocalUsers(cr *argoproj.ClusterArgoCD) error {
	// Retrieve the signing key from the argocd-secret
	argoCDSecret := corev1.Secret{}
	found, err := argoutil.IsObjectFound(r.Client, cr.Spec.ControlPlaneNamespace, common.ArgoCDSecretName, &argoCDSecret)
	if err != nil {
		return fmt.Errorf("error retrieving secret %s: %w", common.ArgoCDSecretName, err)
	}
	if !found {
		return fmt.Errorf("could not find secret %s", common.ArgoCDSecretName)
	}
	signingKey, ok := argoCDSecret.Data["server.secretkey"]
	if !ok {
		return fmt.Errorf("could not find server.secretkey in secret %s", argoCDSecret.Name)
	}

	// Reconcile the local users declared in the argocd CR
	legacyUsers := localUsersInExtraConfig(cr)
	for _, user := range cr.Spec.LocalUsers {
		if legacyUsers[user.Name] {
			continue
		}
		if err := r.reconcileUser(cr, user, signingKey); err != nil {
			return err
		}
	}

	// Delete the secret and token for users that are no longer in the
	// localUsers section of the argocd CR
	if err := r.cleanupLocalUsers(cr); err != nil {
		return err
	}

	return nil
}

func userSecretName(user argoproj.LocalUserSpec) string {
	return user.Name + "-local-user"
}

func tokenRenewalTimerKey(namespace string, name string) string {
	return namespace + "/" + name
}

func (r *ReconcileClusterArgoCD) reconcileUser(cr *argoproj.ClusterArgoCD, user argoproj.LocalUserSpec, signingKey []byte) error {
	// Get the user secret if it exists
	userSecret := corev1.Secret{}
	secretName := userSecretName(user)
	secretExists := true
	err := r.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: cr.Spec.ControlPlaneNamespace}, &userSecret)
	if apierrors.IsNotFound(err) {
		secretExists = false
	} else if err != nil {
		return fmt.Errorf("failed to get or create user secret for user %s: %w", user.Name, err)
	}

	// If the ApiKey setting has been changed to false, delete the secret and
	// the token and the timer.
	if secretExists && user.ApiKey != nil && !*user.ApiKey {
		var err error
		r.LocalUsers.UserTokensLock.protect(func() {
			key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)
			existingTimer, ok := r.LocalUsers.TokenRenewalTimers[key]
			if ok {
				existingTimer.stopped = true
				existingTimer.timer.Stop()
				delete(r.LocalUsers.TokenRenewalTimers, key)
			}
			err = r.cleanupUser(cr, user.Name, userSecret)
		})
		return err
	}

	var tokenDuration time.Duration
	if user.TokenLifetime != "" {
		val, err := time.ParseDuration(user.TokenLifetime)
		if err != nil {
			return fmt.Errorf("failed to parse token lifetime for user %s: %w", user.Name, err)
		}
		tokenDuration = val
	}

	var tokenLifetime string
	if user.TokenLifetime == "" || tokenDuration == 0 {
		tokenLifetime = "0h"
	} else {
		tokenLifetime = user.TokenLifetime
	}

	var autoRenew string
	if user.AutoRenewToken == nil || *user.AutoRenewToken {
		autoRenew = "true"
	} else {
		autoRenew = "false"
	}

	var expAt int64
	if secretExists {
		expAt, err = strconv.ParseInt(string(userSecret.Data[localUserExpiresAt]), 10, 64)
		if err != nil {
			return fmt.Errorf("failed to convert \"%s\" value to int64 from user secret %s: %w", localUserExpiresAt, userSecret.Name, err)
		}
	}

	// If the user secret already exists and neither the token lifetime nor the
	// auto-renew setting have changed, then there is nothing to do other than
	// ensure the timer exists if auto-renew is true.
	tokenLifetimeChanged := secretExists && tokenLifetime != string(userSecret.Data[localUserTokenLifetime])
	autoRenewChanged := secretExists && autoRenew != string(userSecret.Data[localUserAutoRenew])
	if secretExists && !tokenLifetimeChanged && !autoRenewChanged {
		if autoRenew == "true" && tokenDuration > 0 {
			// Sanity test that expAt is defined
			if expAt == 0 {
				return fmt.Errorf("expAt is not defined (has value 0)")
			}

			// If the secret exists and autoRenew is true, but there is no timer for the
			// user, we need to add a timer for the remaining duration. This would
			// happen when the operator is restared.
			r.LocalUsers.UserTokensLock.protect(func() {
				key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)
				if _, ok := r.LocalUsers.TokenRenewalTimers[key]; !ok {
					renewalTimer := &TokenRenewalTimer{}
					renewalTime := time.Unix(expAt, 0)
					timer := time.AfterFunc(time.Until(renewalTime), func() {
						r.LocalUsers.UserTokensLock.protect(func() {
							if renewalTimer.stopped {
								return
							}
							err := r.issueToken(*cr, user, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
							if err != nil {
								log.Error(err, fmt.Sprintf("when trying to re-issue token for user %s", key))
							}
						})
					})
					renewalTimer.timer = timer
					r.LocalUsers.TokenRenewalTimers[key] = renewalTimer
					msg := fmt.Sprintf("Scheduled token renewal for user '%s' to %s", key, renewalTime.Format(time.RFC1123))
					log.Info(msg)
				}
			})
		}
		return nil
	}

	// If auto-renew has been changed to false, delete the timer and if the
	// token lifetime has not changed, there is no need to re-issue the token,
	// so just update the secret.
	if autoRenewChanged && autoRenew == "false" {
		var err error
		r.LocalUsers.UserTokensLock.protect(func() {
			key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)
			existingTimer, ok := r.LocalUsers.TokenRenewalTimers[key]
			if ok {
				existingTimer.stopped = true
				existingTimer.timer.Stop()
				delete(r.LocalUsers.TokenRenewalTimers, key)
			}
			if !tokenLifetimeChanged {
				userSecret.Data[localUserAutoRenew] = []byte(autoRenew)
				argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to false for user", tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name))
				err = r.Update(context.TODO(), &userSecret)
			}
		})
		if !tokenLifetimeChanged {
			return err
		}
	}

	// If auto-renew has been changed to true but the token lifetime has not
	// changed, add a timer for the remaining duration. There is no need to
	// re-issue the token, so just update the secret.
	if autoRenewChanged && autoRenew == "true" && !tokenLifetimeChanged && tokenDuration > 0 {
		// Sanity test that expAt is defined
		if expAt == 0 {
			return fmt.Errorf("expAt is not defined (has value 0)")
		}

		var err error

		r.LocalUsers.UserTokensLock.protect(func() {
			renewalTimer := &TokenRenewalTimer{}
			renewalTime := time.Unix(expAt, 0)
			timer := time.AfterFunc(time.Until(renewalTime), func() {
				r.LocalUsers.UserTokensLock.protect(func() {
					if renewalTimer.stopped {
						return
					}
					err := r.issueToken(*cr, user, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
					if err != nil {
						log.Error(err, fmt.Sprintf("when trying to re-issue token for user %s/%s", cr.Spec.ControlPlaneNamespace, user.Name))
					}
				})
			})
			renewalTimer.timer = timer
			key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)
			r.LocalUsers.TokenRenewalTimers[key] = renewalTimer
			userSecret.Data[localUserAutoRenew] = []byte(autoRenew)
			msg := fmt.Sprintf("Scheduled token renewal for user '%s' to %s", key, renewalTime.Format(time.RFC1123))
			log.Info(msg)

			argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to true")
			err = r.Update(context.TODO(), &userSecret)
		})

		return err
	}

	var explanation string
	if tokenLifetimeChanged {
		explanation = fmt.Sprintf("token lifetime changed from %s to %s for user %s", string(userSecret.Data[localUserTokenLifetime]), tokenLifetime, tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name))
	}
	if autoRenewChanged {
		if tokenLifetimeChanged {
			explanation += ", "
		}
		explanation += fmt.Sprintf("auto-renew changed from %s to %s for user %s", string(userSecret.Data[localUserAutoRenew]), autoRenew, tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name))
	}

	err = nil
	r.LocalUsers.UserTokensLock.protect(func() {
		err = r.issueToken(*cr, user, tokenLifetime, tokenDuration, explanation, signingKey)
	})
	return err
}

// *** THIS METHOD NEEDS TO BE CALLED UNDER THE PROTECTION OF ReconcileClusterArgoCD.UserTokensLock ***
func (r *ReconcileClusterArgoCD) issueToken(cr argoproj.ClusterArgoCD, user argoproj.LocalUserSpec, tokenLifetime string, tokenDuration time.Duration, explanation string, signingKey []byte) error {
	// Get the user secret if it exists, else create a new one
	userSecret := corev1.Secret{}
	secretName := userSecretName(user)
	secretExists := true
	err := r.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: cr.Spec.ControlPlaneNamespace}, &userSecret)
	if apierrors.IsNotFound(err) {
		secretExists = false
		userSecret = *argoutil.NewSecretWithName(&cr, secretName)
		userSecret.Labels[common.ArgoCDKeyComponent] = localUserSecretComponent
		if err := controllerutil.SetControllerReference(&cr, &userSecret, r.Scheme); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("failed to get or create user secret for user %s: %w", user.Name, err)
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

	subject := fmt.Sprintf("%s:%s", user.Name, localUserApiKey)
	jwtToken, err := createJwtToken(subject, issuedAt, tokenDuration, uniqueId, signingKey)
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
		argoutil.LogResourceUpdate(log, &userSecret, explanation)
		if err := r.Update(context.TODO(), &userSecret); err != nil {
			return err
		}
	} else {
		argoutil.LogResourceCreation(log, &userSecret)
		if err := r.Create(context.TODO(), &userSecret); err != nil {
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Spec.ControlPlaneNamespace}, &argoCDSecret)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("accounts.%s.tokens", user.Name)
	argoCDSecret.Data[key] = tokens
	argoutil.LogResourceUpdate(log, &argoCDSecret, "setting token for user account", user.Name)
	if err := r.Update(context.TODO(), &argoCDSecret); err != nil {
		return err
	}

	// Find the timer for the user and update it
	if tokenDuration > 0 && autoRenew == "true" {
		key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)
		existingTimer, ok := r.LocalUsers.TokenRenewalTimers[key]
		if ok {
			existingTimer.stopped = true
			existingTimer.timer.Stop()
			delete(r.LocalUsers.TokenRenewalTimers, key)
		}
		renewalTimer := &TokenRenewalTimer{}
		timer := time.AfterFunc(tokenDuration, func() {
			r.LocalUsers.UserTokensLock.protect(func() {
				if renewalTimer.stopped {
					return
				}
				err := r.issueToken(cr, user, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
				if err != nil {
					log.Error(err, fmt.Sprintf("when trying to re-issue token for user %s", key))
				}
			})
		})
		renewalTimer.timer = timer
		r.LocalUsers.TokenRenewalTimers[tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, user.Name)] = renewalTimer
		msg := fmt.Sprintf("Scheduled token renewal for user '%s' to %s", key, time.Now().Add(tokenDuration).Format(time.RFC1123))
		log.Info(msg)

	}

	return nil
}

func (r *ReconcileClusterArgoCD) cleanupLocalUsers(cr *argoproj.ClusterArgoCD) error {
	// Get a list of the local user secrets
	secrets := corev1.SecretList{}
	options := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDKeyComponent: localUserSecretComponent,
		}),
		Namespace: cr.Spec.ControlPlaneNamespace,
	}
	if err := r.List(context.TODO(), &secrets, &options); err != nil {
		return err
	}

	// Clean up the local user secrets and argocd-secret tokens for any users
	// that aren't declared in the localUsers section of the argocd CR
	for _, secret := range secrets.Items {
		userName := string(secret.Data["user"])
		found := false
		for _, user := range cr.Spec.LocalUsers {
			if userName == user.Name {
				found = true
				break
			}
		}
		if !found {
			var err error
			r.LocalUsers.UserTokensLock.protect(func() {
				key := tokenRenewalTimerKey(cr.Spec.ControlPlaneNamespace, userName)
				existingTimer, ok := r.LocalUsers.TokenRenewalTimers[key]
				if ok {
					existingTimer.stopped = true
					existingTimer.timer.Stop()
					delete(r.LocalUsers.TokenRenewalTimers, key)
				}
				err = r.cleanupUser(cr, userName, secret)
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileClusterArgoCD) cleanupUser(cr *argoproj.ClusterArgoCD, userName string, secret corev1.Secret) error {
	// Delete the secret
	argoutil.LogResourceDeletion(log, &secret, "deleting secret for user", userName)
	if err := r.Delete(context.TODO(), &secret); err != nil {
		return err
	}

	// Delete the token from the argocd-secret provided the user isn't in the
	// extraConfig and using an apiKey
	value := cr.Spec.ExtraConfig["accounts."+userName]
	if !strings.Contains(value, localUserApiKey) {
		argoCDSecret := corev1.Secret{}
		if err := r.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Spec.ControlPlaneNamespace}, &argoCDSecret); err != nil {
			return err
		}

		key := fmt.Sprintf("accounts.%s.tokens", userName)
		if _, ok := argoCDSecret.Data[key]; ok {
			argoutil.LogResourceUpdate(log, &argoCDSecret, "deleting token for user", userName)
			delete(argoCDSecret.Data, key)
			if err := r.Update(context.TODO(), &argoCDSecret); err != nil {
				return err
			}
		}
	}

	return nil
}

func createJwtToken(subject string, issuedAt time.Time, expiresIn time.Duration, id string, serverSignature []byte) (string, error) {
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
func localUsersInExtraConfig(cr *argoproj.ClusterArgoCD) map[string]bool {
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

func (r *ReconcileClusterArgoCD) cleanupNamespaceTokenTimers(namespace string) {
	r.LocalUsers.UserTokensLock.protect(func() {
		log.Info(fmt.Sprintf("removing local user token renewal timers for namespace \"%s\"", namespace))
		prefix := namespace + "/"
		for key, timer := range r.LocalUsers.TokenRenewalTimers {
			if strings.HasPrefix(key, prefix) {
				timer.stopped = true
				timer.timer.Stop()
				delete(r.LocalUsers.TokenRenewalTimers, key)
			}
		}
	})
}
