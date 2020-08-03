// Copyright 2019 Argo CD Operator Developers
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
	"crypto/rsa"
	"crypto/x509"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// hasArgoTLSChanged will return true if the Argo TLS certificate or key have changed.
func hasArgoTLSChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualCert := string(actual.Data[common.ArgoCDKeyTLSCert])
	actualKey := string(actual.Data[common.ArgoCDKeyTLSPrivateKey])
	expectedCert := string(expected.Data[common.ArgoCDKeyTLSCert])
	expectedKey := string(expected.Data[common.ArgoCDKeyTLSPrivateKey])

	if actualCert != expectedCert || actualKey != expectedKey {
		log.Info("tls secret has changed")
		return true
	}
	return false
}

// newCASecret creates a new CA secret with the given suffix for the given ArgoCD.
func newCASecret(meta metav1.ObjectMeta) (*corev1.Secret, error) {
	secret := resources.NewTLSSecret(meta, "ca")

	key, err := common.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := common.NewSelfSignedCACertificate(key)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       common.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: common.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ArgoClusterReconciler) reconcileCAConfigMap(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, getCAConfigMapName(cr))
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := resources.NewSecretWithSuffix(cr.ObjectMeta, common.ArgoCDCASuffix)
	if !resources.IsObjectFound(r.Client, cr.Namespace, caSecret.Name, caSecret) {
		log.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ArgoClusterReconciler) reconcileTLSCerts(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, common.ArgoCDTLSCertsConfigMapName)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = getInitialTLSCerts(cr)

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// newCertificateSecret creates a new secret using the given name suffix for the given TLS certificate.
func newCertificateSecret(suffix string, caCert *x509.Certificate, caKey *rsa.PrivateKey, cr *v1alpha1.ArgoCD) (*corev1.Secret, error) {
	secret := resources.NewTLSSecret(cr.ObjectMeta, suffix)

	key, err := common.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &common.CertConfig{
		CertName:     secret.Name,
		CertType:     common.ClientAndServingCert,
		CommonName:   secret.Name,
		Organization: []string{cr.Namespace},
	}

	dnsNames := []string{
		cr.Name,
		common.NameWithSuffix(cr.ObjectMeta, "grpc"),
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.Name, cr.Namespace),
	}

	if cr.Spec.Grafana.Enabled {
		dnsNames = append(dnsNames, getGrafanaHost(cr))
	}
	if cr.Spec.Prometheus.Enabled {
		dnsNames = append(dnsNames, getPrometheusHost(cr))
	}

	cert, err := common.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       common.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: common.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ArgoClusterReconciler) reconcileCertificateAuthority(cr *v1alpha1.ArgoCD) error {
	log.Info("reconciling CA secret")
	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	log.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}
