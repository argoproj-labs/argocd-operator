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

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var (
	certName = "trust-cert"
)

// reconcileTrustCerts will ensure that Secret for Trust Certificate exist.
func (r *ReconcileArgoCD) reconcileTrustCert(cr *argoprojv1a1.ArgoCD) error {
	cert := cr.Spec.TrustCerts

	actual := argoutil.NewSecretWithSuffix(cr, certName)
	desired := argoutil.NewSecretWithSuffix(cr, certName)
	desired.Data = map[string][]byte{
		"cert.pem": []byte(cert.Cert),
	}
	// Add component label to secret
	desired.ObjectMeta.Labels[common.ArgoCDKeyComponent] = certName

	if argoutil.IsObjectFound(r.Client, cr.Namespace, actual.Name, actual) {
		// Secret with Certificate exists but enabled flag has been set to false, delete the Secret
		if !cr.Spec.TrustCerts.Enabled {
			if err := r.Client.Delete(context.TODO(), actual); err != nil {
				return err
			}
			return nil
		}

		// Check if actual and desired Cert have changes
		if hasTrustCertSpecChanged(actual, desired) {
			return r.Client.Update(context.TODO(), desired)
		}
		return nil // Certs found, do nothing
	}

	if !cr.Spec.TrustCerts.Enabled {
		return nil // Trust Certs not enabled, do nothing.
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		return err
	}

	return r.Client.Create(context.TODO(), desired)

}

// hasTrustCertSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasTrustCertSpecChanged(actual, desired *v1.Secret) bool {
	// Actual Secret don't have cert.pem data
	actualData, ok := actual.Data["cert.pem"]
	if !ok {
		return true
	}

	// Actual Secret cert.pem data not equal with desired
	if string(actualData) != string(desired.Data["cert.pem"]) {
		return true
	}

	return false
}

// addTrustCertData added certs volume inside Object
func addTrustCertData(cr *argoprojv1a1.ArgoCD, object *appsv1.Deployment) {
	// Trust Certs disabled. Nothing to do
	if !cr.Spec.TrustCerts.Enabled {
		return
	}

	for i, container := range object.Spec.Template.Spec.Containers {
		object.Spec.Template.Spec.Containers[i].VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      certName,
			MountPath: "/etc/ssl/certs",
		})
	}

	object.Spec.Template.Spec.Volumes = append(object.Spec.Template.Spec.Volumes, v1.Volume{
		Name: certName,
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: fmt.Sprintf("%s-%s", cr.Name, certName),
			},
		},
	})
}
