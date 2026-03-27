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
	"fmt"
	"testing"
	"time"

	"github.com/argoproj-labs/argocd-operator/internal/settings"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func createResources(cr *argoproj.ArgoCD, expect *assert.Assertions) *ReconcileArgoCD {
	var err error

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)

	r := makeTestReconciler(cl, sch, testclient.NewClientset())

	// Create and get the argocd-secret. The argocd-secret needs to exist before
	// the tests call reconcileLocalUsers()
	clusterSecret := argoutil.NewSecretWithSuffix(cr, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(cr, "tls")
	err = r.Create(context.TODO(), clusterSecret)
	expect.NoError(err)
	err = r.Create(context.TODO(), tlsSecret)
	expect.NoError(err)
	err = r.reconcileArgoSecret(cr)
	expect.NoError(err)

	return r
}

func cleanupAllTokenTimers(r *ReconcileArgoCD) {

	r.LocalUsers.lock.Lock()
	defer r.LocalUsers.lock.Unlock()

	for key, timer := range r.LocalUsers.tokenRenewalTimers {
		timer.stopped = true
		timer.timer.Stop()
		delete(r.LocalUsers.tokenRenewalTimers, key)
	}
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created and that it
	// contains the correct info

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
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

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
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

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Reconcile again to ensure nothing changes

	originalAPIToken := userSecret.Data["apiToken"]

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.Equal(originalAPIToken, userSecret.Data["apiToken"])

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	expect.True(timer == r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]) // testing pointer equality

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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created and that it
	// contains the correct info

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
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

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
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

	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	// Reconcile again to ensure nothing changes

	originalAPIToken := userSecret.Data["apiToken"]

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.Equal(originalAPIToken, userSecret.Data["apiToken"])
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
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
	expect.NoError(r.reconcileLocalUsers(*cr))

	originalUserSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &originalUserSecret)
	expect.NoError(err)

	argocdSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	originalTokens := argocdSecret.Data["accounts.alice.tokens"]

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Update the token lifetime and check that the token was reissued

	cr.Spec.LocalUsers[0].TokenLifetime = "2h"
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	updatedUserSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &updatedUserSecret)
	expect.NoError(err)

	expect.NotEqual(originalUserSecret.Data["apiToken"], updatedUserSecret.Data["apiToken"])
	expect.Equal("2h", string(updatedUserSecret.Data["tokenLifetime"]))

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEqual(originalTokens, argocdSecret.Data["accounts.alice.tokens"])

	// Check there is a new renewal timer

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer1 := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(*cr))
	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR and reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the user secret was deleted
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was removed
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(*cr))
	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR, add them to the extraConfig and
	// reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	cr.Spec.ExtraConfig = map[string]string{
		"accounts.alice": "login, apiKey",
	}
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the user secret was deleted
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was not removed
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile to create the user secret and add the user's token to the argocd-secret
	expect.NoError(r.reconcileLocalUsers(*cr))
	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the user from the argocd CR, add them to the extraConfig and
	// reconcile again
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{}
	cr.Spec.ExtraConfig = map[string]string{
		"accounts.alice": "login",
	}
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the user secret was deleted
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Check that the user's token was removed
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Check the renewal timer was removed

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile to create the artifacts

	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)

	// Retrieve the token from the user secret

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(2200 * time.Millisecond) // slightly longer than the timeout, but not too much longer, so as to avoid second refresh

	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err = jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims = token.Claims.(jwt.MapClaims)

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens = []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	expect.True(r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
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

	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that no timer was created

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
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

	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Get the token from the user secret

	userSecret := corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew off

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(false)
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the timer was deleted

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped

	// Check that the token has not changed

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal(apiToken, string(userSecret.Data["apiToken"]))

}

func TestReconcileArgoCD_reconcileArgoLocalUsersTurnOnAutoRenew(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	t.Log("Start with autorenew off")

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
	err := r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	serverSecretKey := argocdSecret.Data["server.secretkey"]

	t.Log("Reconcile to create the artifacts")

	expect.NoError(r.reconcileLocalUsers(*cr))

	t.Log("Check that a timer was not created")

	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	t.Log("Check the data in the user secret")

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	t.Log("Check the data in the argocd secret")

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)

	argocdSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
	expect.Equal(int64(claims["exp"].(float64)), userTokens[0].ExpiresAt)

	t.Log("Turn autorenew on")

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(true)
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	t.Log("Check that a timer was added")

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	t.Log("Check the api token in the user secret was not updated")

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	expect.Equal(apiToken, string(userSecret.Data["apiToken"]))

	t.Log("Check the data in the argocd secret was not updated")

	argocdSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens1 := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens1)
	expect.NoError(err)

	expect.Len(userTokens1, 1)
	expect.Equal(userTokens[0].ID, userTokens1[0].ID)
	expect.Equal(userTokens[0].IssuedAt, userTokens1[0].IssuedAt)
	expect.Equal(userTokens[0].ExpiresAt, userTokens1[0].ExpiresAt)

	// Wait for the timer to expire and check that it updated the secrets
	t.Log("Wait for the timer to expire and check that it updated the secrets")

	time.Sleep(2200 * time.Millisecond) // slightly longer than the timeout, but not too much longer, so as to avoid second refresh

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err = jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	}, jwt.WithoutClaimsValidation())
	expect.NoError(err)
	claims = token.Claims.(jwt.MapClaims)

	argocdSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
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

	t.Log("Check that the timer was re-created")

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	expect.True(r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
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

	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that a timer was not created

	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	// Check the data in the user secret

	userSecret := corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("false", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew on and change the token lifetime

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(true)
	cr.Spec.LocalUsers[0].TokenLifetime = "2s"
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that a timer was added

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Check that the token was re-issued

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken1 := string(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, apiToken1)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(2200 * time.Millisecond) // slightly longer than the timeout, but not too much longer, so as to avoid second refresh

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))
	expect.NotEqual(apiToken1, string(userSecret.Data["apiToken"]))

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	expect.True(r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
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

	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the timer was created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Get the token from the user secret

	userSecret := corev1.Secret{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("true", string(userSecret.Data["autoRenew"]))
	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Turn autorenew off and change the token lifetime

	cr.Spec.LocalUsers[0].AutoRenewToken = boolPtr(false)
	cr.Spec.LocalUsers[0].TokenLifetime = "0s"
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// Check that the timer was deleted

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
	expect.True(timer.stopped)
	expect.False(timer.timer.Stop()) // check that the timer was stopped

	// Check that the token was re-issued

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile and then check that the user secret was created

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Change the ApiKey setting to false and reconcile again

	cr.Spec.LocalUsers[0].ApiKey = boolPtr(false)
	expect.NoError(r.Update(context.TODO(), cr))
	expect.NoError(r.reconcileLocalUsers(*cr))

	// The secret should have been deleted

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// The timer should have been removed

	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	// The entry for the user should have been removed from the argocd secret

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
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
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile and then check that the user secret was created

	expect.NoError(r.reconcileLocalUsers(*cr))

	userSecret := corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	// Get the api token from the user secret

	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken := string(userSecret.Data["apiToken"])

	// Check that there's a timer to renew the token

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Remove the timer (simulate restart of operator) and reconcile again

	timer.stopped = true
	timer.timer.Stop()
	delete(r.LocalUsers.tokenRenewalTimers, cr.Namespace+"/alice")
	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	expect.NoError(r.reconcileLocalUsers(*cr))

	// The timer should have been recreated

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer = r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)

	// Retrieve the api token from the user secret and check it has not changed

	userSecret = corev1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotEmpty(userSecret.Data["apiToken"])
	apiToken1 := string(userSecret.Data["apiToken"])
	expect.Equal(apiToken, apiToken1)

	// Wait for the timer to expire and check that it updated the secrets

	time.Sleep(2200 * time.Millisecond) // slightly longer than the timeout, but not too much longer, so as to avoid second refresh

	err = r.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.NotNil(userSecret.Data["apiToken"])
	expect.NotEqual(apiToken, string(userSecret.Data["apiToken"]))

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)

	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(claims["jti"], userTokens[0].ID)

	// Check that the timer was re-created

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	expect.True(r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"] != timer) // testing pointer inequality
}

func TestReconcileArgoCD_cleanupNamespaceTokenTimers(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	r.LocalUsers.tokenRenewalTimers["foo/alice"] = &tokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.tokenRenewalTimers["foo/bob"] = &tokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.tokenRenewalTimers["bar/alice"] = &tokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}
	r.LocalUsers.tokenRenewalTimers["bar/bob"] = &tokenRenewalTimer{
		timer: time.NewTimer(1 * time.Hour),
	}

	r.cleanupNamespaceTokenTimers("foo")

	expect.Len(r.LocalUsers.tokenRenewalTimers, 2)
	keys := make([]string, 0, 2)
	for key := range r.LocalUsers.tokenRenewalTimers {
		keys = append(keys, key)
	}
	expect.Contains(keys, "bar/alice")
	expect.Contains(keys, "bar/bob")

	r.cleanupNamespaceTokenTimers("bar")
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretDoesNotExist_CreatesTokenAndSecret(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}

	ctx := context.TODO()

	// Retrieve the signing key so we can verify the JWT later
	argocdSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	serverSecretKey := argocdSecret.Data["server.secretkey"]

	// Reconcile a new user with no pre-existing secret
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Verify the user secret was created with expected metadata
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))

	expect.Equal("alice", string(userSecret.Data[localUserUser]))
	expect.Equal("1h", string(userSecret.Data[localUserTokenLifetime]))
	expect.Equal("true", string(userSecret.Data[localUserAutoRenew]))
	expect.NotEmpty(userSecret.Data[localUserApiToken])
	expect.NotEmpty(userSecret.Data[localUserExpiresAt])

	// Parse the JWT and verify its claims are correct
	token, err := jwt.Parse(string(userSecret.Data[localUserApiToken]), func(token *jwt.Token) (interface{}, error) {
		return serverSecretKey, nil
	})
	expect.NoError(err)
	claims := token.Claims.(jwt.MapClaims)
	expect.Equal("argocd", claims["iss"])
	expect.Equal("alice:apiKey", claims["sub"])
	expect.NotNil(claims["exp"])

	// Verify the argocd-secret was updated with the user's token info
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	userTokens := []settings.Token{}
	expect.NoError(json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens))
	expect.Len(userTokens, 1)

	// Verify a renewal timer was scheduled for the 1h lifetime
	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)
}

func TestReconcileLocalUser_SecretDoesNotExist_NoTokenLifetime_CreatesWithInfiniteLifetime(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// TokenLifetime is omitted, which means the token should never expire
	user := argoproj.LocalUserSpec{
		Name: "alice",
	}

	ctx := context.TODO()

	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Verify the secret was created with the normalized "0h" lifetime
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))

	expect.Equal("0h", string(userSecret.Data[localUserTokenLifetime]))
	expect.NotEmpty(userSecret.Data[localUserApiToken])
	expect.Equal("true", string(userSecret.Data[localUserAutoRenew]))
	expect.Equal("0", string(userSecret.Data[localUserExpiresAt]))

	// No renewal timer should be scheduled for a non-expiring token
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretDoesNotExist_EnabledFalse_CleansUp(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// User is explicitly disabled -- no secret should be created
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
		Enabled:       boolPtr(false),
	}

	ctx := context.TODO()

	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// No user secret should exist
	userSecret := corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// argocd-secret should have no token entry for this user
	argocdSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretDoesNotExist_ApiKeyFalse_CleansUp(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	// apiKey is false -- user exists for login only, no API token needed
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
		ApiKey:        boolPtr(false),
	}

	ctx := context.TODO()

	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// No user secret should be created when apiKey is false
	userSecret := corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretExists_EnabledSetToFalse_DeletesSecretAndToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// First reconcile creates the user secret and renewal timer
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]

	// Disable the user and reconcile again
	user.Enabled = boolPtr(false)
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// User secret should have been deleted
	err := r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Token entry should have been removed from argocd-secret
	argocdSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Renewal timer should have been stopped and removed
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
	expect.True(timer.stopped)
}

func TestReconcileLocalUser_SecretExists_ApiKeySetToFalse_DeletesSecretAndToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// First reconcile creates the user secret and renewal timer
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]

	// Set apiKey to false and reconcile -- the API token is no longer needed
	user.ApiKey = boolPtr(false)
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// User secret should have been deleted
	err := r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.True(apierrors.IsNotFound(err))

	// Token entry should have been removed from argocd-secret
	argocdSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Renewal timer should have been stopped and removed
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
	expect.True(timer.stopped)
}

func TestReconcileLocalUser_SecretExists_TokenLifetimeChanged_ReissuesToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create initial token with 1h lifetime
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Capture the original token and timer for later comparison
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])
	originalTimer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]

	// Change the lifetime to 2h -- this should invalidate and reissue the token
	user.TokenLifetime = "2h"
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Verify a new token was issued with the updated lifetime
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.NotEqual(originalToken, string(userSecret.Data[localUserApiToken]))
	expect.Equal("2h", string(userSecret.Data[localUserTokenLifetime]))

	// Verify the renewal timer was replaced (new pointer) and the old one stopped
	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	newTimer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.True(originalTimer != newTimer)
	expect.True(originalTimer.stopped)
}

func TestReconcileLocalUser_SecretExists_ExpAtZeroWithNonZeroDuration_RecreatesToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create initial token with 1h lifetime
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])

	// Set the expAt field to 0, simulating an invalid state where the
	// secret claims no expiration but the user has a non-zero lifetime
	userSecret.Data[localUserExpiresAt] = []byte("0")
	expect.NoError(r.Update(ctx, &userSecret))

	// Reconcile should detect the inconsistency and recreate the token
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.NotEqual(originalToken, string(userSecret.Data[localUserApiToken]))
}

func TestReconcileLocalUser_SecretExists_TokenExpired_AutoRenewTrue_RenewsToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create initial token (autoRenew defaults to true)
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])

	// Manually set expAt to one hour in the past to simulate an expired token
	expiredTime := time.Now().Add(-1 * time.Hour).Unix()
	userSecret.Data[localUserExpiresAt] = []byte(fmt.Sprintf("%d", expiredTime))
	expect.NoError(r.Update(ctx, &userSecret))

	// Reconcile should detect the expiration and issue a new token
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.NotEqual(originalToken, string(userSecret.Data[localUserApiToken]))

	// A new renewal timer should have been scheduled for the fresh token
	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)
}

func TestReconcileLocalUser_SecretExists_AutoRenewChangedTrueToFalse_UpdatesSecretStopsTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create token with default autoRenew (true)
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])
	expect.Equal("true", string(userSecret.Data[localUserAutoRenew]))

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]

	// Turn off auto-renew
	user.AutoRenewToken = boolPtr(false)
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// The secret's autoRenew field should be updated, but the token itself
	// should remain unchanged (no reason to reissue)
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal("false", string(userSecret.Data[localUserAutoRenew]))
	expect.Equal(originalToken, string(userSecret.Data[localUserApiToken]))

	// The renewal timer should have been stopped and removed
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
	expect.True(timer.stopped)
}

func TestReconcileLocalUser_SecretExists_AutoRenewChangedFalseToTrue_UpdatesSecretStartsTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create token with autoRenew explicitly off
	user := argoproj.LocalUserSpec{
		Name:           "alice",
		TokenLifetime:  "1h",
		AutoRenewToken: boolPtr(false),
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])
	expect.Equal("false", string(userSecret.Data[localUserAutoRenew]))
	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	// Turn on auto-renew
	user.AutoRenewToken = boolPtr(true)
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// The secret's autoRenew field should be updated, but the token itself
	// should remain unchanged (only the renewal policy changed)
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal("true", string(userSecret.Data[localUserAutoRenew]))
	expect.Equal(originalToken, string(userSecret.Data[localUserApiToken]))

	// A renewal timer should now be scheduled
	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	timer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(timer)
	expect.False(timer.stopped)
}

func TestReconcileLocalUser_SecretExists_TimerMissing_SchedulesTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create initial token and timer
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	originalTimer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]

	// Manually remove the timer to simulate an operator restart where
	// in-memory timers are lost but the secret still exists on disk
	originalTimer.stopped = true
	originalTimer.timer.Stop()
	delete(r.LocalUsers.tokenRenewalTimers, cr.Namespace+"/alice")
	expect.Empty(r.LocalUsers.tokenRenewalTimers)

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])

	// Reconcile again -- should detect the missing timer
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Token should not have changed since it hasn't expired
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal(originalToken, string(userSecret.Data[localUserApiToken]))

	// Timer should have been recreated
	expect.Len(r.LocalUsers.tokenRenewalTimers, 1)
	newTimer := r.LocalUsers.tokenRenewalTimers[cr.Namespace+"/alice"]
	expect.NotNil(newTimer)
	expect.False(newTimer.stopped)
}

func TestReconcileLocalUser_TokenLifetimeZero_NoTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// "0" is treated as infinite lifetime (no expiration)
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "0",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Token should be created but with the normalized "0h" infinite lifetime
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal("0h", string(userSecret.Data[localUserTokenLifetime]))
	expect.NotEmpty(userSecret.Data[localUserApiToken])

	// No renewal timer should be scheduled for a non-expiring token
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretDoesNotExist_AutoRenewFalse_NoTimer(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create a user with autoRenew off -- token should be created but with
	// no renewal timer, so it will expire without being replaced
	user := argoproj.LocalUserSpec{
		Name:           "alice",
		TokenLifetime:  "1h",
		AutoRenewToken: boolPtr(false),
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Verify the secret was created with autoRenew=false
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal("false", string(userSecret.Data[localUserAutoRenew]))
	expect.NotEmpty(userSecret.Data[localUserApiToken])
	expect.NotEqual("0", string(userSecret.Data[localUserExpiresAt]))

	// No renewal timer should exist
	expect.Empty(r.LocalUsers.tokenRenewalTimers)
}

func TestReconcileLocalUser_SecretExists_ArgoCDSecretMissingTokenEntry_RestoresEntry(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create initial token with 1h lifetime — this populates both the
	// local user secret and the accounts.alice.tokens entry in argocd-secret
	user := argoproj.LocalUserSpec{
		Name:          "alice",
		TokenLifetime: "1h",
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// Verify both secrets were set up correctly
	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])
	expect.NotEmpty(originalToken)

	argocdSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	// Simulate the argocd-secret losing the accounts.alice.tokens entry
	// (e.g. another controller or manual edit removed it)
	delete(argocdSecret.Data, "accounts.alice.tokens")
	expect.NoError(r.Update(ctx, &argocdSecret))

	// Confirm the entry is gone
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.Empty(argocdSecret.Data["accounts.alice.tokens"])

	// Reconcile again — the local user secret still exists and is valid,
	// but the argocd-secret is missing the token entry. The reconciler
	// should detect this and restore the entry.
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	// The local user secret's API token should have been regenerated, due to missing entry in Argo CD Secret
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.NotEqual(originalToken, string(userSecret.Data[localUserApiToken]))

	// The argocd-secret should now have the accounts.alice.tokens entry
	// restored
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret))
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"],
		"expected argocd-secret to have accounts.alice.tokens restored, but it was empty")

	userTokens := []settings.Token{}
	expect.NoError(json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens))
	expect.Len(userTokens, 1)
}

func TestReconcileLocalUser_SecretExists_TokenExpired_AutoRenewFalse_DoesNotRenew(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	expect := assert.New(t)

	cr := makeTestArgoCD()
	r := createResources(cr, expect)
	defer cleanupAllTokenTimers(r)

	ctx := context.TODO()

	// Create token with autoRenew explicitly off
	user := argoproj.LocalUserSpec{
		Name:           "alice",
		TokenLifetime:  "1h",
		AutoRenewToken: boolPtr(false),
	}
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	userSecret := corev1.Secret{}
	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	originalToken := string(userSecret.Data[localUserApiToken])
	expect.NotEqual("0", string(userSecret.Data[localUserExpiresAt]))

	// Manually set expAt to one hour in the past to simulate an expired token
	expiredTime := time.Now().Add(-1 * time.Hour).Unix()
	userSecret.Data[localUserExpiresAt] = []byte(fmt.Sprintf("%d", expiredTime))
	expect.NoError(r.Update(ctx, &userSecret))

	// Reconcile should leave the expired token as-is because autoRenew is false
	expect.NoError(r.reconcileLocalUser(ctx, *cr, user))

	expect.NoError(r.Get(ctx, types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret))
	expect.Equal(originalToken, string(userSecret.Data[localUserApiToken]))
}
