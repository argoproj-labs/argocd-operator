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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type semaphore struct {
	sync.Mutex
}

func (s *semaphore) protect(code func()) {
	s.Lock()
	defer s.Unlock()
	code()
}

var (
	expiringTokens     = map[string]*time.Timer{}
	expiringTokensLock semaphore
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
	// localUsers section of theargocd CR
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
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: cr.Namespace}, &userSecret)
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
		expiringTokensLock.protect(func() {
			key := cr.Namespace + "/" + user.Name
			existingTimer, ok := expiringTokens[key]
			if ok {
				existingTimer.Stop()
				delete(expiringTokens, key)
			}
		})

		if err := r.cleanupUser(cr, user.Name, userSecret); err != nil {
			return err
		}
		return nil
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

	expAt, err := strconv.ParseInt(string(userSecret.Data["expAt"]), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert \"expAt\" value to int64 from user secret %s: %w", userSecret.Name, err)
	}

	// If the use secret already exists and neither the token lifetime nor the
	// auto-renew setting have changed, then there is nothing to do other than
	// ensure the timer exists if auto-renew is true.
	tokenLifetimeChanged := secretExists && tokenLifetime != string(userSecret.Data["tokenLifetime"])
	autoRenewChanged := secretExists && autoRenew != string(userSecret.Data["autoRenew"])
	if !tokenLifetimeChanged && !autoRenewChanged {
		if secretExists && autoRenew == "true" {
			// If the secret exists and autoRenew is true, but there is no timer for the
			// user, we need to add a timer for the remaining duration. This would
			// happen when the operator is restared.
			expiringTokensLock.protect(func() {
				key := cr.Namespace + "/" + user.Name
				if _, ok := expiringTokens[key]; !ok {
					when := time.Until(time.Unix(expAt, 0))
					timer := time.AfterFunc(when, func() {
						err := r.issueToken(*cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
						if err != nil {
							log.Error(err, "when trying to re-issue token for user", user.Name)
						}
					})
					expiringTokens[key] = timer
				}
			})
		}
		return nil
	}

	// If auto-renew has been changed to false, delete the timer and if the
	// token lifetime has not changed, there is no need to re-issue the token,
	// so just update the secret.
	if autoRenewChanged && autoRenew == "false" {
		expiringTokensLock.protect(func() {
			key := cr.Namespace + "/" + user.Name
			existingTimer, ok := expiringTokens[key]
			if ok {
				existingTimer.Stop()
				delete(expiringTokens, key)
			}
		})

		if !tokenLifetimeChanged {
			userSecret.Data["autoRenew"] = []byte(autoRenew)
			argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to false for user", cr.Namespace+"/"+user.Name)
			if err := r.Client.Update(context.TODO(), &userSecret); err != nil {
				return err
			}
			return nil
		}
	}

	// If auto-renew has been changed to true but the token lifetime has not
	// changed, add a timer for the remaining duration. There is no need to
	// re-issue the token, so just update the secret.
	if autoRenewChanged && autoRenew == "true" && !tokenLifetimeChanged {
		userSecret.Data["autoRenew"] = []byte(autoRenew)
		argoutil.LogResourceUpdate(log, &userSecret, "autoRenew set to true")
		if err := r.Client.Update(context.TODO(), &userSecret); err != nil {
			return err
		}

		expiringTokensLock.protect(func() {
			when := time.Until(time.Unix(expAt, 0))
			timer := time.AfterFunc(when, func() {
				err := r.issueToken(*cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
				if err != nil {
					log.Error(err, "when trying to re-issue token for user", cr.Namespace+"/"+user.Name)
				}
			})
			expiringTokens[cr.Namespace+"/"+user.Name] = timer
		})

		return nil
	}

	var explanation string
	if tokenLifetimeChanged {
		explanation = fmt.Sprintf("token lifetime changed from %s to %s for user %s", string(userSecret.Data["tokenLifetime"]), tokenLifetime, cr.Namespace+"/"+user.Name)
	}
	if autoRenewChanged {
		if tokenLifetimeChanged {
			explanation += ", "
		}
		explanation += fmt.Sprintf("auto-renew changed from %s to %s for user %s", string(userSecret.Data["autoRenew"]), autoRenew, cr.Namespace+"/"+user.Name)
	}

	err = r.issueToken(*cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, explanation, signingKey)
	if err != nil {
		return err
	}

	return nil
}

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

	subject := fmt.Sprintf("%s:%s", user.Name, "apiKey")
	jwtToken, err := createJwtToken(subject, issuedAt, tokenDuration, uniqueId, signingKey)
	if err != nil {
		return fmt.Errorf("error creating token for user %s: %w", user.Name, err)
	}

	// We store the TokenLifetime so we can tell if it's been changed in the
	// ArgoCD CR
	userSecret.Data = map[string][]byte{
		"user":          []byte(user.Name),
		"ID":            []byte(uniqueId),
		"expAt":         []byte(strconv.FormatInt(expiresAt, 10)),
		"tokenLifetime": []byte(tokenLifetime),
		"autoRenew":     []byte(strconv.FormatBool(*user.AutoRenewToken)),
		"apiToken":      []byte(jwtToken),
	}

	// Create or update the local user secret
	if secretExists {
		argoutil.LogResourceUpdate(log, &userSecret, explanation)
		if err := r.Client.Update(context.TODO(), &userSecret); err != nil {
			return err
		}
	} else {
		if err := controllerutil.SetControllerReference(&cr, &userSecret, r.Scheme); err != nil {
			return err
		}
		argoutil.LogResourceCreation(log, &userSecret)
		if err := r.Client.Create(context.TODO(), &userSecret); err != nil {
			return err
		}
	}

	// Find the timer for the user and update it
	if tokenDuration > 0 {
		expiringTokensLock.protect(func() {
			key := cr.Namespace + "/" + user.Name
			existingTimer, ok := expiringTokens[key]
			if ok {
				existingTimer.Stop()
				delete(expiringTokens, key)
			}
			timer := time.AfterFunc(tokenDuration, func() {
				err := r.issueToken(cr, user, userSecret, secretExists, tokenLifetime, tokenDuration, "token automatically re-issued after expiration", signingKey)
				if err != nil {
					log.Error(err, "when trying to re-issue token for user", user.Name)
				}
			})
			expiringTokens[cr.Namespace+"/"+user.Name] = timer
		})
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
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("accounts.%s.tokens", user.Name)
	argoCDSecret.Data[key] = tokens
	argoutil.LogResourceUpdate(log, &argoCDSecret, "setting token for user account", user.Name)
	if err := r.Client.Update(context.TODO(), &argoCDSecret); err != nil {
		return err
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
	if err := r.Client.List(context.TODO(), &secrets, &options); err != nil {
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
			if err := r.cleanupUser(cr, userName, secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileArgoCD) cleanupUser(cr *argoproj.ArgoCD, userName string, secret corev1.Secret) error {
	// Delete the secret
	argoutil.LogResourceDeletion(log, &secret, "deleting secret for user", userName)
	if err := r.Client.Delete(context.TODO(), &secret); err != nil {
		return err
	}

	// Delete the token from the argocd-secret provided the user isn't in the
	// extraConfig and using an apiKey
	value := cr.Spec.ExtraConfig["accounts."+userName]
	if !strings.Contains(value, "apiKey") {
		argoCDSecret := corev1.Secret{}
		if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: cr.Namespace}, &argoCDSecret); err != nil {
			return err
		}

		key := fmt.Sprintf("accounts.%s.tokens", userName)
		if _, ok := argoCDSecret.Data[key]; ok {
			argoutil.LogResourceUpdate(log, &argoCDSecret, "deleting token for user", userName)
			delete(argoCDSecret.Data, key)
			if err := r.Client.Update(context.TODO(), &argoCDSecret); err != nil {
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

func (r *ReconcileArgoCD) ExpiringTokens(cr *argoproj.ArgoCD) ([]settings.Token, error) {
	argoCDSecret := corev1.Secret{}
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, common.ArgoCDSecretName, &argoCDSecret) {
		return nil, fmt.Errorf("could not find secret %s", common.ArgoCDSecretName)
	}

	var tokens []settings.Token

	legacyUsers := localUsersInExtraConfig(cr)
	for _, user := range cr.Spec.LocalUsers {
		if legacyUsers[user.Name] {
			continue
		}
		if user.ApiKey != nil && !*user.ApiKey {
			continue
		}
		if user.TokenLifetime == "" {
			continue
		}

		key := fmt.Sprintf("accounts.%s.tokens", user.Name)
		value := argoCDSecret.Data[key]

		var accountTokens []settings.Token
		if err := json.Unmarshal(value, &accountTokens); err != nil {
			return nil, err
		}
		if len(accountTokens) != 1 {
			return nil, fmt.Errorf("expected 1 token for user %s, got %d", user.Name, len(accountTokens))
		}

		tokens = append(tokens, accountTokens[0])
	}

	slices.SortFunc(tokens, func(a settings.Token, b settings.Token) int {
		value := a.ExpiresAt - b.ExpiresAt
		if value < 0 {
			return -1
		} else if value > 0 {
			return 1
		} else {
			return 0
		}
	})

	return tokens, nil
}
