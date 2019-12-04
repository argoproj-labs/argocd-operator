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
	"io/ioutil"
	"path/filepath"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultKnownHosts = `bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H	
`
)

func getCAConfigMapName(cr *argoproj.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return nameWithSuffix(ArgoCDCASuffix, cr)
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(cr *argoproj.ArgoCD) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func newConfigMapWithName(name string, cr *argoproj.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// newConfigMapWithName creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ArgCD.
func newConfigMapWithSuffix(suffix string, cr *argoproj.ArgoCD) *corev1.ConfigMap {
	return newConfigMapWithName(fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, suffix), cr)
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ReconcileArgoCD) reconcileConfigMaps(cr *argoproj.ArgoCD) error {
	if err := r.reconcileConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileRBAC(cr); err != nil {
		return err
	}

	if err := r.reconcileSSHKnownHosts(cr); err != nil {
		return err
	}

	if err := r.reconcileTLSCerts(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaDashboards(cr); err != nil {
		return err
	}

	return nil
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ReconcileArgoCD) reconcileCAConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(getCAConfigMapName(cr), cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := newSecretWithSuffix(ArgoCDCASuffix, cr)
	caSecret, err := r.getSecret(caSecret.Name, cr)
	if err != nil {
		return err
	}

	cm.Data = map[string]string{
		TLSCACertKey: string(caSecret.Data[TLSCertKey]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ReconcileArgoCD) reconcileConfiguration(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(ArgoCDConfigMapName, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaConfiguration(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(ArgoCDGrafanaConfigMapSuffix, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	grafanaConfig := GrafanaConfig{
		Security: GrafanaSecurityConfig{
			AdminUser:     ArgoCDDefaultGrafanaAdminUsername,
			AdminPassword: "secret",
			SecretKey:     "SW2YcwTIb9zpOOhoPsMm",
		},
	} // TODO: Pull these values from CR!

	data, err := loadGrafanaConfigs()
	if err != nil {
		return err
	}

	tmpls, err := loadGrafanaTemplates(&grafanaConfig)
	if err != nil {
		return err
	}

	for key, val := range tmpls {
		data[key] = val
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaDashboards(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(ArgoCDGrafanaDashboardConfigMapSuffix, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	pattern := filepath.Join(getGrafanaConfigPath(), "dashboards/*.json")
	dashboards, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	data := make(map[string]string)
	for _, f := range dashboards {
		dashboard, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(dashboard)
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ReconcileArgoCD) reconcileRBAC(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(ArgoCDRBACConfigMapName, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ReconcileArgoCD) reconcileSSHKnownHosts(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(ArgoCDKnownHostsConfigMapName, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	cm.Data = map[string]string{
		ArgoCDKeySSHKnownHosts: defaultKnownHosts,
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ReconcileArgoCD) reconcileTLSCerts(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(ArgoCDTLSCertsConfigMapName, cr)
	if r.isObjectFound(cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}
