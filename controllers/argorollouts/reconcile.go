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

package argorollouts

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

func (r *ArgoRolloutsReconciler) reconcileRolloutsController(cr *argoprojv1a1.ArgoRollouts) error {

	log.Info("reconciling rollouts serviceaccount")
	sa, err := r.reconcileRolloutsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts roles")
	role, err := r.reconcileRolloutsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts role bindings")
	if err := r.reconcileRolloutsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts secret")
	if err := r.reconcileRolloutsSecrets(cr); err != nil {
		return err
	}

	log.Info("reconciling rollouts deployment")
	if err := r.reconcileRolloutsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts service")
	if err := r.reconcileRolloutsService(cr); err != nil {
		return err
	}

	return nil
}

// setResourceWatches will register Watches for each of the supported Resources.
func setResourceWatches(bld *builder.Builder) *builder.Builder {
	// Watch for changes to primary resource ArgoRollouts
	bld.For(&argoproj.ArgoRollouts{})

	// Watch for changes to ConfigMap sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.ConfigMap{})

	// Watch for changes to Secret sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.Secret{})

	// Watch for changes to Service sub-resources owned by ArgoRollouts.
	bld.Owns(&corev1.Service{})

	// Watch for changes to Deployment sub-resources owned by ArgoRollouts.
	bld.Owns(&appsv1.Deployment{})

	// Watch for changes to Role sub-resources owned by ArgoRollouts.
	bld.Owns(&v1.Role{})

	// Watch for changes to RoleBinding sub-resources owned by ArgoRollouts.
	bld.Owns(&v1.RoleBinding{})

	return bld
}
