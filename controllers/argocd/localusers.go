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
	"sync"
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
	localUserApiKey        = "apiKey"
	localUserApiToken      = "apiToken"
	localUserAutoRenew     = "autoRenew"
	localUserExpiresAt     = "expAt"
	localUserID            = "ID"
	localUserTokenLifetime = "tokenLifetime"
	localUserUser          = "user"
)

type lock struct {
	lock sync.Mutex
}

func (l *lock) protect(code func()) {
	l.lock.Lock()
	defer l.lock.Unlock()
	code()
}

type tokenRenewalTimer struct {
	timer *time.Timer
	stop  bool
}

var (
	tokenRenewalTimers = map[string]*tokenRenewalTimer{}
	userTokensLock     lock
)

func (r *ReconcileArgoCD) reconcileLocalUsers(cr *argoproj.ArgoCD) error {
	// Retrieve the signing key from the argocd-secret
	argoCDSecret := corev1.Secret{}
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, common.ArgoCDSecretName, &argoCDSecret) {
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

func (r *ReconcileArgoCD) reconcileUser(cr *argoproj.ArgoCD, user argoproj.LocalUserSpec, signingKey []byte) error {
	var err error
	secretExists := true

	// Get the user secret if it exists, else create a new one
	userSecret := corev1.Secret{}
	secretName := user.Name + "-local-user"
	err = r.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: cr.Namespace}, &userSecret)
	if apierrors.IsNotFound(err) {
		secretExists = false
		userSecret = *argoutil.NewSecretWithName(cr, secretName)
		userSecret.Labels[common.ArgoCDKeyComponent] = "local-users"
	} else if err != nil {
		return err
	}

	// If the ApiKey setting has been changed to false, delete the secret and
	// the token and the timer.
	if secretExists && user.ApiKey != nil && !*user.ApiKey {
		var err error
		userTokensLock.protect(func() {
			key := cr.Namespace + "/" + user.Name
			existingTimer, ok := tokenRenewalTimers[key]
			if ok {
				existingTimer.stop = true
				existingTimer.timer.Stop()
				delete(tokenRenewalTimers, key)
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

	// If the use secret already exists and neither the token lifetime nor the
	// auto-renew setting have changed, then there is nothing to do other than
	// ensure the timer exists if auto-renew is true.
	tokenLifetimeChanged := secretExists && tokenLifetime != string(userSecret.Data[localUserTokenLifetime])
	autoRenewChanged := secretExists && autoRenew != string(userSecret.Data[localUserAutoRenew])
	if secretExists && !tokenLifetimeChanged && !autoRenewChanged {
		if autoRenew == "true" && tokenDuration > 0 {
			// If the secret exists and autoRenew is true, but there is no timer for the
			// user, we need to add a timer for the remaining duration. This would
			// happen when the operator is restared.
			userTokensLock.protect(func() {
				key := cr.Namespace + "/" + user.Name
				if _, ok := tokenRenewalTimers[key]; !ok {
					renewalTimer := &tokenRenewalTimer{}
					when := time.Until(time.Unix(expAt, 0))
					timer := time.AfterFunc(when, func() {
						userTokensLock.protect(func() {
							if renewalTimer.stop {
								return
							}
							err := r.issueToken(*cr, user, userSecret, true, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
							if err != nil {
								log.Error(err, "when trying to re-issue token for user", user.Name)
							}
						})
					})
					renewalTimer.timer = timer
					tokenRenewalTimers[key] = renewalTimer
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
		userTokensLock.protect(func() {
			key := cr.Namespace + "/" + user.Name
			existingTimer, ok := tokenRenewalTimers[key]
			if ok {
				existingTimer.stop = true
				existingTimer.timer.Stop()
				delete(tokenRenewalTimers, key)
			}
			if !tokenLifetimeChanged {
				userSecret.Data[localUserAutoRenew] = []byte(autoRenew)
				argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to false for user", cr.Namespace+"/"+user.Name)
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
		var err error

		userTokensLock.protect(func() {
			renewalTimer := &tokenRenewalTimer{}
			when := time.Until(time.Unix(expAt, 0))
			timer := time.AfterFunc(when, func() {
				userTokensLock.protect(func() {
					if renewalTimer.stop {
						return
					}
					err := r.issueToken(*cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
					if err != nil {
						log.Error(err, "when trying to re-issue token for user", cr.Namespace+"/"+user.Name)
					}
				})
			})
			renewalTimer.timer = timer
			tokenRenewalTimers[cr.Namespace+"/"+user.Name] = renewalTimer
			userSecret.Data[localUserAutoRenew] = []byte(autoRenew)
			argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to true")
			err = r.Update(context.TODO(), &userSecret)
		})

		return err
	}

	var explanation string
	if tokenLifetimeChanged {
		explanation = fmt.Sprintf("token lifetime changed from %s to %s for user %s", string(userSecret.Data[localUserTokenLifetime]), tokenLifetime, cr.Namespace+"/"+user.Name)
	}
	if autoRenewChanged {
		if tokenLifetimeChanged {
			explanation += ", "
		}
		explanation += fmt.Sprintf("auto-renew changed from %s to %s for user %s", string(userSecret.Data[localUserAutoRenew]), autoRenew, cr.Namespace+"/"+user.Name)
	}

	err = nil
	userTokensLock.protect(func() {
		err = r.issueToken(*cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, explanation, signingKey)
	})
	return nil
}

// *** THIS METHOD NEEDS TO BE CALLED UNDER THE PROTECTION OF userTokensLock ***
func (r *ReconcileArgoCD) issueToken(cr argoproj.ArgoCD, user argoproj.LocalUserSpec, userSecret corev1.Secret, secretExists bool, tokenLifetime string, tokenDuration time.Duration, explanation string, signingKey []byte) error {

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
		localUserID:            []byte(uniqueId),
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
		if err := controllerutil.SetControllerReference(&cr, &userSecret, r.Scheme); err != nil {
			return err
		}
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret)
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
		key := cr.Namespace + "/" + user.Name
		existingTimer, ok := tokenRenewalTimers[key]
		if ok {
			existingTimer.stop = true
			existingTimer.timer.Stop()
			delete(tokenRenewalTimers, key)
		}
		renewalTimer := &tokenRenewalTimer{}
		timer := time.AfterFunc(tokenDuration, func() {
			userTokensLock.protect(func() {
				if renewalTimer.stop {
					return
				}
				err := r.issueToken(cr, user, userSecret, true, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
				if err != nil {
					log.Error(err, "when trying to re-issue token for user", user.Name)
				}
			})
		})
		renewalTimer.timer = timer
		tokenRenewalTimers[cr.Namespace+"/"+user.Name] = renewalTimer
	}

	return nil
}

func (r *ReconcileArgoCD) cleanupLocalUsers(cr *argoproj.ArgoCD) error {
	// Get a list of the local user secrets
	secrets := corev1.SecretList{}
	options := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			common.ArgoCDKeyComponent: "local-users",
		}),
		Namespace: cr.Namespace,
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
			userTokensLock.protect(func() {
				key := cr.Namespace + "/" + userName
				existingTimer, ok := tokenRenewalTimers[key]
				if ok {
					existingTimer.stop = true
					existingTimer.timer.Stop()
					delete(tokenRenewalTimers, key)
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

func (r *ReconcileArgoCD) cleanupUser(cr *argoproj.ArgoCD, userName string, secret corev1.Secret) error {
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
		if err := r.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret); err != nil {
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
func localUsersInExtraConfig(cr *argoproj.ArgoCD) map[string]bool {
	localUsers := make(map[string]bool)
	for k := range cr.Spec.ExtraConfig {
		if strings.HasPrefix(k, "accounts.") && !strings.HasSuffix(k, ".enabled") {
			user := k[len("accounts."):]
			localUsers[user] = true
		}
	}
	return localUsers
}

func cleanupNamespaceTokenTimers(namespace string) {
	userTokensLock.protect(func() {
		log.Info(fmt.Sprintf("removing local user token renewal timers for namespace \"%s\"", namespace))
		prefix := namespace + "/"
		for key, timer := range tokenRenewalTimers {
			if strings.HasPrefix(key, prefix) {
				timer.stop = true
				timer.timer.Stop()
				delete(tokenRenewalTimers, key)
			}
		}
	})
}

// For use by test code
func cleanupAllTokenTimers() {
	userTokensLock.protect(func() {
		for key, timer := range tokenRenewalTimers {
			timer.stop = true
			timer.timer.Stop()
			delete(tokenRenewalTimers, key)
		}
	})
}
