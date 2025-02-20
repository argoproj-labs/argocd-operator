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
	"strings"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileArgoCD) reconcileLocalUsers(cr *argoproj.ArgoCD) error {
	argoCDSecret := argoutil.NewSecretWithName(cr, common.ArgoCDSecretName)
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, argoCDSecret.Name, argoCDSecret) {
		return fmt.Errorf("could not find argocd-secret")
	}

	signingKey, ok := argoCDSecret.Data["server.secretkey"]
	if !ok {
		return fmt.Errorf("could not find server.secretkey in argocd-secret")
	}

	legacyUsers := localUsersInExtraConfig(cr)

	for _, user := range cr.Spec.LocalUsers {
		if legacyUsers[user.Name] {
			continue
		}
		if user.ApiKey != nil && !*user.ApiKey {
			continue
		}
		if err := r.reconcileUser(cr, user, argoCDSecret, signingKey); err != nil {
			return err
		}
	}

	// Delete the secret and token for users that are no longer in the argocd CR
	if err := r.cleanupLocalUsers(cr); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileUser(cr *argoproj.ArgoCD, user argoproj.LocalUserSpec, argoCDSecret *corev1.Secret, signingKey []byte) error {
	// Create the values from which the token is generated

	var uniqueId string
	if u, err := uuid.NewRandom(); err != nil {
		return fmt.Errorf("failed to generate unique ID for token for user %s: %w", user.Name, err)
	} else {
		uniqueId = u.String()
	}

	issuedAt := time.Now()

	var expiresIn time.Duration
	if user.TokenLifetime != "" {
		val, err := time.ParseDuration(user.TokenLifetime)
		if err != nil {
			return fmt.Errorf("failed to parse token lifetime for user %s: %w", user.Name, err)
		}
		expiresIn = val
	}

	// Create the secret containing the API token for this user

	secret := argoutil.NewSecretWithName(cr, user.Name+"-local-user")
	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil
	}
	secret.Labels[common.ArgoCDKeyComponent] = "local-users"

	subject := fmt.Sprintf("%s:%s", user.Name, "apiKey")
	jwtToken, err := createJwtToken(subject, issuedAt, expiresIn, uniqueId, signingKey)
	if err != nil {
		return fmt.Errorf("error creating token for user %s: %w", user.Name, err)
	}

	secret.Data = map[string][]byte{
		"user":     []byte(user.Name),
		"ID":       []byte(uniqueId),
		"apiToken": []byte(jwtToken),
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}

	argoutil.LogResourceCreation(log, secret)
	if err := r.Client.Create(context.TODO(), secret); err != nil {
		return err
	}

	// Add the token to the argocd-secret

	var expiresAt int64
	if expiresIn > 0 {
		expiresAt = issuedAt.Add(expiresIn).Unix()
	}

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

	key := fmt.Sprintf("accounts.%s.tokens", user.Name)
	value := string(tokens)
	argoCDSecret.Data[key] = []byte(value)
	argoutil.LogResourceUpdate(log, argoCDSecret, "adding token for user account", user.Name)
	if err := r.Client.Update(context.TODO(), argoCDSecret); err != nil {
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

	// Delete any users that aren't declared in the localUsers section of the
	// argocd CR
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
			if err := r.cleanupUser(userName, secret); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileArgoCD) cleanupUser(userName string, secret corev1.Secret) error {
	namespace := secret.Namespace

	// Delete the secret
	argoutil.LogResourceDeletion(log, &secret, "deleting secret for user", userName)
	if err := r.Client.Delete(context.TODO(), &secret); err != nil {
		return err
	}

	// Delete the token from the argocd-secret
	argoCDSecret := corev1.Secret{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDSecretName, Namespace: namespace}, &argoCDSecret); err != nil {
		return err
	}
	key := fmt.Sprintf("accounts.%s.tokens", userName)
	delete(argoCDSecret.Data, key)
	argoutil.LogResourceUpdate(log, &argoCDSecret, "deleting token for user", userName)
	if err := r.Client.Update(context.TODO(), &argoCDSecret); err != nil {
		return err
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
