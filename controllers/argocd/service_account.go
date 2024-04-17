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

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newServiceAccountWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newServiceAccountWithName(name string, cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	sa := newServiceAccount(cr)
	sa.ObjectMeta.Name = getServiceAccountName(cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

func getServiceAccountName(crName, name string) string {
	return fmt.Sprintf("%s-%s", crName, name)
}

// reconcileServiceAccounts will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileServiceAccounts(cr *argoproj.ArgoCD) error {
	params := getPolicyRuleList(r.Client, cr)

	for _, param := range params {
		if err := r.reconcileServiceAccountPermissions(param.name, param.policyRule, cr); err != nil {
			return err
		}
	}

	clusterParams := getPolicyRuleClusterRoleList()

	for _, clusterParam := range clusterParams {
		if err := r.reconcileServiceAccountClusterPermissions(clusterParam.name, clusterParam.policyRule, cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileServiceAccountClusterPermissions(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	var role *v1.ClusterRole
	var err error

	_, err = r.reconcileServiceAccount(name, cr)
	if err != nil {
		return err
	}

	if role, err = r.reconcileClusterRole(name, rules, cr); err != nil {
		return err
	}

	return r.reconcileClusterRoleBinding(name, role, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccountPermissions(name string, rules []v1.PolicyRule, cr *argoproj.ArgoCD) error {
	return r.reconcileRoleBinding(name, rules, cr)
}

func (r *ReconcileArgoCD) reconcileServiceAccount(name string, cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {
	sa := newServiceAccountWithName(name, cr)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
			return sa, nil // Dex installation not requested, do nothing
		}
		exists = false
	}
	if exists {
		if name == common.ArgoCDDexServerComponent && !UseDex(cr) {
			// Delete any existing Service Account created for Dex since dex is disabled
			log.Info("deleting the existing Dex service account because dex uninstallation requested")
			return sa, r.Client.Delete(context.TODO(), sa)
		}
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	log.Info(fmt.Sprintf("creating serviceaccount %s for Argo CD instance %s in namespace %s", sa.Name, cr.Name, cr.Namespace))

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, nil
}
