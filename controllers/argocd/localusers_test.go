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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileArgoLocalUsersCreate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name: "alice",
		},
	}

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Create and get the argocd-secret. The argocd-secret needs to exist before
	// the test calls reconcileLocalUsers()
	clusterSecret := argoutil.NewSecretWithSuffix(cr, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(cr, "tls")
	err = r.Client.Create(context.TODO(), clusterSecret)
	expect.NoError(err)
	r.Client.Create(context.TODO(), tlsSecret)
	expect.NoError(err)
	err = r.reconcileArgoSecret(cr)
	expect.NoError(err)
	argocdSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)

	// Reconcile and then check that the user secret was created and that it
	// contains the correct token and ID

	expect.NoError(r.reconcileLocalUsers(cr))

	userSecret := corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "alice-local-user", Namespace: cr.Namespace}, &userSecret)
	expect.NoError(err)

	expect.Equal("local-users", userSecret.Labels[common.ArgoCDKeyComponent])
	expect.Equal("alice", string(userSecret.Data["user"]))
	expect.NotEmpty(userSecret.Data["ID"])
	expect.NotEmpty(userSecret.Data["apiToken"])

	token, err := jwt.Parse(string(userSecret.Data["apiToken"]), func(token *jwt.Token) (interface{}, error) {
		return argocdSecret.Data["server.secretkey"], nil
	})
	expect.NoError(err)

	claims := token.Claims.(jwt.MapClaims)
	expect.Equal("argocd", claims["iss"])
	expect.Equal("alice:apiKey", claims["sub"])
	expect.Equal(string(userSecret.Data["ID"]), claims["jti"])

	// Check that the argocd-secret has been updated with the user's token info

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: cr.Namespace}, &argocdSecret)
	expect.NoError(err)
	expect.NotEmpty(argocdSecret.Data["accounts.alice.tokens"])

	userTokens := []settings.Token{}
	err = json.Unmarshal(argocdSecret.Data["accounts.alice.tokens"], &userTokens)
	expect.NoError(err)

	expect.Len(userTokens, 1)
	expect.Equal(string(userSecret.Data["ID"]), userTokens[0].ID)
	expect.Equal(int64(claims["iat"].(float64)), userTokens[0].IssuedAt)
}

func TestReconcileArgoCD_reconcileArgoLocalUsersDelete(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	var err error

	expect := assert.New(t)

	cr := makeTestArgoCD()
	cr.Spec.LocalUsers = []argoproj.LocalUserSpec{
		{
			Name: "alice",
		},
	}

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Create and get the argocd-secret. The argocd-secret needs to exist before
	// the test calls reconcileLocalUsers()
	clusterSecret := argoutil.NewSecretWithSuffix(cr, "cluster")
	clusterSecret.Data = map[string][]byte{common.ArgoCDKeyAdminPassword: []byte("something")}
	tlsSecret := argoutil.NewSecretWithSuffix(cr, "tls")
	err = r.Client.Create(context.TODO(), clusterSecret)
	expect.NoError(err)
	r.Client.Create(context.TODO(), tlsSecret)
	expect.NoError(err)
	err = r.reconcileArgoSecret(cr)
	expect.NoError(err)
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
}
