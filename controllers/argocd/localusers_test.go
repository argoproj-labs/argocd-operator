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
	"encoding/json"
	"testing"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func createResources(cr *argoproj.ArgoCD, expect *assert.Assertions) *ReconcileArgoCD {
	var err error

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)

	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Create and get the argocd-secret. The argocd-secret needs to exist before
	// the tests call reconcileLocalUsers()
	clusterSecret := argoutil.NewSecretWithSuffix(cr, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(cr, "tls")
	err = r.Client.Create(context.TODO(), clusterSecret)
	expect.NoError(err)
	r.Client.Create(context.TODO(), tlsSecret)
	expect.NoError(err)
	err = r.reconcileArgoSecret(cr)
	expect.NoError(err)

	return r
}

func cleanupAllTokenTimers(r *ReconcileArgoCD) {
	r.LocalUsers.UserTokensLock.protect(func() {
		for key, timer := range r.LocalUsers.TokenRenewalTimers {
			timer.stopped = true
			timer.timer.Stop()
			delete(r.LocalUsers.TokenRenewalTimers, key)
		}
	})
}

func TestReconcileArgoCD_reconcileArgoLocalUsersCreate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created and that it
	// contains the correct info

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal("1h", string(userSecret.Data["tokenLifetime"]))

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)

	claims := token.Claims.(jwt.MapClaims)
	expect.Equal("argocd", claims["iss"])
	expect.Equal("alice:apiKey", claims["sub"])

	// Check that the argocd-secret has been updated with the user's token info

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Reconcile again to ensure nothing changes

	originalAPIToken := userSecret.Data["apiToken"]

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.Equal(originalAPIToken, userSecret.Data["apiToken"])

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	expect.True(timer == r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]) // testing pointer equality

}

func TestReconcileArgoCD_reconcileArgoLocalUsersCreateWithDefaultTokenLifetime(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name: "alice",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created and that it
	// contains the correct info

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal("0h", string(userSecret.Data["tokenLifetime"]))

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)

	claims := token.Claims.(jwt.MapClaims)
	expect.Equal("argocd", claims["iss"])
	expect.Equal("alice:apiKey", claims["sub"])

	// Check that the argocd-secret has been updated with the user's token info

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(0), userTokens[0].ExpiresAt)

	// Check no token renewal timer was created

	expect.Empty(r.LocalUsers.TokenRenewalTimers)

	// Reconcile again to ensure nothing changes

	originalAPIToken := userSecret.Data["apiToken"]

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.Equal(originalAPIToken, userSecret.Data["apiToken"])
	expect.Empty(r.LocalUsers.TokenRenewalTimers)
}

func TestReconcileArgoCD_reconcileArgoLocalUsersUpdateTokenLifetime(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	// Reconcile to create the user secret and update the argocd-secret

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)
	expect.NoError(r.reconcileLocalUsers(cr))

	originalUserSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &originalUserSecret)
	expect.NoError(err)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	originalTokens := argocdSecret.Data["accounts.alice.tokens"]

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Update the token lifetime and check that the token was reissued

	cr.Spec.LocalUsers[0].TokenLifetime = "2h"
	expect.NoError(r.reconcileLocalUsers(cr))

	updatedUserSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &updatedUserSecret)
	expect.NoError(err)

	expect.NotEqual(originalUserSecret.Data["apiToken"], updatedUserSecret.Data["apiToken"])
	expect.Equal("2h", string(updatedUserSecret.Data["tokenLifetime"]))

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEqual(originalTokens, argocdSecret.Data["accounts.alice.tokens"])

	// Check there is a new renewal timer

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer1 := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer1)
	expect.False(timer1.stopped)

	expect.True(timer != timer1) // testing pointer inequality
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped
}

func TestReconcileArgoCD_reconcileArgoLocalUsersDelete(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(cr))
	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR and reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the user secret was deleted
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was removed
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped
}

func TestReconcileArgoCD_reconcileArgoLocalUsersDeleteWithExtraConfigAPIKey(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(cr))
	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR, add them to the extraConfig and
	// reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	cr.Spec.ExtraConfig = map[string]string{
		"accounts.alice": "login, apiKey",
	}
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the user secret was deleted
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was not removed
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped
}

func TestReconcileArgoCD_reconcileArgoLocalUsersDeleteWithExtraConfigLogin(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(cr))
	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR, add them to the extraConfig and
	// reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	cr.Spec.ExtraConfig = map[string]string{
		"accounts.alice": "login",
	}
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the user secret was deleted
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was removed
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped
}

func TestReconcileArgoCD_reconcileArgoLocalUsersBasicAutoRenew(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "2s",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)

	// Retrieve the token from the user secret

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)

	// Check that the argocd-secret has been updated with the user's token info

	argocdSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(3 * time.Second)

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err = jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims = token.Claims.(jwt.MapClaims)

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens = []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	expect.True(r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
}

func TestReconcileArgoCD_reconcileArgoLocalUsersAutoRenewOnTokenNeverExpires(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	// Set autorenew on and infinite token lifetime

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:           "alice",
			TokenLifetime:  "0s",
			AutoRenewToken: boolPtr(true),
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that no timer was created

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
}

func TestReconcileArgoCD_reconcileArgoLocalUsersTurnOffAutoRenew(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	// Start with autorenew on

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "10s",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Get the token from the user secret

	userSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew off

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(false)
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the timer was deleted

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped

	// Check that the token has not changed

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal(apiToken, string(userSecret.Data["apiToken"]))

}

func TestReconcileArgoCD_reconcileArgoLocalUsersTurnOnAutoRenew(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	// Start with autorenew off

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:           "alice",
			TokenLifetime:  "2s",
			AutoRenewToken: boolPtr(false),
		},
	}
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that a timer was not created

	expect.Empty(r.LocalUsers.TokenRenewalTimers)

	// Check the data in the user secret

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Check the data in the argocd secret

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)

	argocdSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Turn autorenew on

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(true)
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that a timer was added

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Check the api token in the user secret was not updated

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal(apiToken, string(userSecret.Data["apiToken"]))

	// Check the data in the argocd secret was not updated

	argocdSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens1 := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens1)
	expect.NoError(err)

	expect.Len(userTokens1, 1)
	expect.Equal(userTokens[0].ID, userTokens1[0].ID)
	expect.Equal(userTokens[0].IssuedAt, userTokens1[0].IssuedAt)
	expect.Equal(userTokens[0].ExpiresAt, userTokens1[0].ExpiresAt)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(3 * time.Second)

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err = jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims = token.Claims.(jwt.MapClaims)

	argocdSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens2 := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens2)
	expect.NoError(err)

	expect.Len(userTokens2, 1)
	expect.Equal(claims["jti"], userTokens2[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens2[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens2[0].ExpiresAt)

	expect.NotEqual(userTokens[0].ID, userTokens2[0].ID)
	expect.NotEqual(userTokens[0].IssuedAt, userTokens2[0].IssuedAt)
	expect.NotEqual(userTokens[0].ExpiresAt, userTokens2[0].ExpiresAt)

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	expect.True(r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
}

func TestReconcileArgoCD_reconcileArgoLocalUsersTurnOnAutoRenewChangeTokenLifetime(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	// Start with autorenew off

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:           "alice",
			TokenLifetime:  "0s",
			AutoRenewToken: boolPtr(false),
		},
	}
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that a timer was not created

	expect.Empty(r.LocalUsers.TokenRenewalTimers)

	// Check the data in the user secret

	userSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew on and change the token lifetime

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(true)
	cr.Spec.LocalUsers[0].TokenLifetime = "2s"
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that a timer was added

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Check that the token was re-issued

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken1 := string(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, apiToken1)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(3 * time.Second)

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))
	expect.NotEqual(apiToken1, string(userSecret.Data["apiToken"]))

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	expect.True(r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
}

func TestReconcileArgoCD_reconcileArgoLocalUsersTurnOffAutoRenewChangeTokenLifetime(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	// Start with autorenew on

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "10s",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Get the token from the user secret

	userSecret := corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew off and change the token lifetime

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(false)
	cr.Spec.LocalUsers[0].TokenLifetime = "0s"
	expect.NoError(r.reconcileLocalUsers(cr))

	// Check that the timer was deleted

	expect.Empty(r.LocalUsers.TokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped

	// Check that the token was re-issued

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

}

func TestReconcileArgoCD_reconcileArgoLocalUsersSetAPIKeyFalse(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "1h",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile and then check that the user secret was created

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Change the ApiKey setting to false and reconcile again

	cr.Spec.LocalUsers[0].ApiKey = boolPtr(false)
	expect.NoError(r.reconcileLocalUsers(cr))

	// The secret should have been deleted

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// The timer should have been removed

	expect.Empty(r.LocalUsers.TokenRenewalTimers)

	// The entry for the user should have been removed from the argocd secret

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])
}

func TestReconcileArgoCD_reconcileArgoLocalUsersRecreateTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name:          "alice",
			TokenLifetime: "2s",
		},
	}

	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	// Get the api token from the user secret

	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer := r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the timer (simulate restart of operator) and reconcile again

	timer.stopped = true
	timer.timer.Stop()
	delete(r.LocalUsers.TokenRenewalTimers, cr.Namespace+"/alice")
	expect.Empty(r.LocalUsers.TokenRenewalTimers)

	expect.NoError(r.reconcileLocalUsers(cr))

	// The timer should have been recreated

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	timer = r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Retrieve the api token from the user secret and check it has not changed

	userSecret = corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken1 := string(userSecret.Data["apiToken"])
	expect.Equal(apiToken, apiToken1)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(3 * time.Second)

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.TokenRenewalTimers, 1)
	expect.True(r.LocalUsers.TokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
}

func TestReconcileArgoCD_cleanupNamespaceTokenTimers(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	r.LocalUsers.TokenRenewalTimers["foo/alice"] = &TokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.TokenRenewalTimers["foo/bob"] = &TokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.TokenRenewalTimers["bar/alice"] = &TokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.TokenRenewalTimers["bar/bob"] = &TokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}

	r.cleanupNamespaceTokenTimers("foo")

	expect.Len(r.LocalUsers.TokenRenewalTimers, 2)
	keys := make([]string, 0, 2)
	for key := range r.LocalUsers.TokenRenewalTimers {
		keys = append(keys, key)
	}
	expect.Contains(keys, "bar/alice")
	expect.Contains(keys, "bar/bob")

	r.cleanupNamespaceTokenTimers("bar")
	expect.Empty(r.LocalUsers.TokenRenewalTimers)
}

func TestReconcileArgoCD_invalidUserInExtraConfig(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	cr.Spec.ExtraConfig = map[string]string{
		"accounts.alice":     "login",
		"accounts.bob.bogus": "login",
	}

	ec := localUsersInExtraConfig(cr)
	expect.Len(ec, 1)
	keys := make([]string, 0, 1)
	for key := range ec {
		keys = append(keys, key)
	}
	expect.Contains(keys, "alice")
}
