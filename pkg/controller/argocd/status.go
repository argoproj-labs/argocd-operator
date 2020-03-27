// Copyright 2020 ArgoCD Operator Developers
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

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
)

// reconcileStatus will ensure that all of the Status properties are updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatus(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileStatusPhase(cr); err != nil {
		return err
	}
	if err := r.reconcileStatusServer(cr); err != nil {
		return err
	}
	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusPhase(cr *argoprojv1a1.ArgoCD) error {
	phase := "Unknown"

	if cr.Status.Server == "Running" {
		phase = "Available"
	} else {
		phase = "Pending"
	}

	if cr.Status.Phase != phase {
		cr.Status.Phase = phase
		return r.client.Status().Update(context.TODO(), cr)
	}
	return nil
}

// reconcileStatusPhase will ensure that the Status Phase is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusServer(cr *argoprojv1a1.ArgoCD) error {
	status := "Unknown"

	deploy := newDeploymentWithSuffix("server", "server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Running"
		}
	}

	if cr.Status.Server != status {
		cr.Status.Server = status
		return r.client.Status().Update(context.TODO(), cr)
	}
	return nil
}
