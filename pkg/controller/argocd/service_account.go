// Copyright 2019 ArgoCD Operator Developers
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
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ReconcileArgoCD) getDexOAuthRedirectURI(cr *argoprojv1a1.ArgoCD) string {
	uri := r.getArgoServerURI(cr)
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath
}

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(cr *argoprojv1a1.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newServiceAccountWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newServiceAccountWithName(name string, cr *argoprojv1a1.ArgoCD) *corev1.ServiceAccount {
	sa := newServiceAccount(cr)
	sa.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileServiceAccounts(cr *argoprojv1a1.ArgoCD) error {

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDServerComponent, policyRuleForServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDApplicationControllerComponent, policyRuleForApplicationController(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDRedisHAComponent, policyRuleForRedisHa(), cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDServerComponent, cr); err != nil {
		return err
	}

	if err := r.reconcileServiceAccountClusterPermissions(common.ArgoCDApplicationControllerComponent, cr); err != nil {
		return err
	}

	// specialized handling for dex

	if err := r.reconcileDexServiceAccount(cr); err != nil {
		return err
	}

	return nil
}

// reconcileDexServiceAccount will ensure that the Dex ServiceAccount is configured properly for OpenShift OAuth.
func (r *ReconcileArgoCD) reconcileDexServiceAccount(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Dex.OpenShiftOAuth {
		return nil // OpenShift OAuth not enabled, move along...
	}

	log.Info("oauth enabled, configuring dex service account")
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.client, cr.Namespace, sa.Name, sa); err != nil {
		return err
	}

	// Get the OAuth redirect URI that should be used.
	uri := r.getDexOAuthRedirectURI(cr)
	log.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.ObjectMeta.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	log.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.ObjectMeta.Annotations = ann

	return r.client.Update(context.TODO(), sa)
}

func (r *ReconcileArgoCD) reconcileServiceAccountClusterPermissions(name string, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.ClusterRole
	var sa *corev1.ServiceAccount
	var err error

	if role, err = r.getClusterRole(name); err != nil {
		return err
	}

	sa, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	return r.reconcileClusterRoleBinding(name, role, sa, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccountPermissions(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	var role *v1.Role
	var sa *corev1.ServiceAccount
	var err error

	if role, err = r.reconcileRole(name, rules, cr); err != nil {
		return err
	}

	sa, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	return r.reconcileRoleBinding(name, role, sa, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccount(name string, cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := newServiceAccountWithName(name, cr)

	exists := true
	if err := argoutil.FetchObject(r.client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}
	if exists {
		return sa, nil
	}

	err := r.client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}
