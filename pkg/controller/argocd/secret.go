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
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	tlsutil "github.com/operator-framework/operator-sdk/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newCASecret creates a new CA secret with the given suffix for the given ArgoCD.
func newCASecret(cr *argoprojv1a1.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, "ca")

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := argoutil.NewSelfSignedCACertificate(key)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// newCertificateSecret creates a new secret using the given name suffix for the given TLS certificate.
func newCertificateSecret(suffix string, caCert *x509.Certificate, caKey *rsa.PrivateKey, cr *argoprojv1a1.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, suffix)

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &tlsutil.CertConfig{
		CertName:     secret.Name,
		CertType:     tlsutil.ClientAndServingCert,
		CommonName:   secret.Name,
		Organization: []string{cr.ObjectMeta.Namespace},
	}

	dnsNames := []string{
		cr.ObjectMeta.Name,
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.ObjectMeta.Name, cr.ObjectMeta.Namespace),
	}

	cert, err := argoutil.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// reconcileArgoSecret will ensure that the ArgoCD Secret is present.
func (r *ReconcileArgoCD) reconcileArgoSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, common.ArgoCDSecretName)
	found := argoutil.IsObjectFound(r.client, cr.Namespace, secret.Name, secret)
	if found {
		// TODO: If secret changed, update configs and reload cluster as needed.
		return nil // Secret found, do nothing
	}

	adminPassword, err := generateArgoAdminPassword()
	if err != nil {
		return err
	}
	adminPasswordModified := time.Now().UTC().Format(time.RFC3339)

	sessionKey, err := generateArgoServerSessionKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      adminPassword,
		common.ArgoCDKeyAdminPasswordMTime: []byte(adminPasswordModified),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
	}

	// TODO: handle TLS cert/key, webhook secrets and additional users

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileArgoTLSSecret ensures the TLS Secret is created for the ArgoCD Service.
func (r *ReconcileArgoCD) reconcileArgoTLSSecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, "tls")
	found := argoutil.IsObjectFound(r.client, cr.Namespace, secret.Name, secret)
	if found {
		return nil // Secret found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	caSecret, err := argoutil.FetchSecret(r.client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := argoutil.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := argoutil.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = newCertificateSecret("tls", caCert, caKey, cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), secret)
}

// reconcileCASecret ensures the CA Secret is created.
func (r *ReconcileArgoCD) reconcileCASecret(cr *argoprojv1a1.ArgoCD) error {
	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	if argoutil.IsObjectFound(r.client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := newCASecret(cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ReconcileArgoCD) reconcileGrafanaSecret(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")
	if argoutil.IsObjectFound(r.client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	adminPassword, err := getGrafanaAdminPassword()
	if err != nil {
		return err
	}

	secretKey, err := getGrafanaSecretKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyGrafanaAdminUsername: []byte(common.ArgoCDDefaultGrafanaAdminUsername),
		common.ArgoCDKeyGrafanaAdminPassword: adminPassword,
		common.ArgoCDKeyGrafanaSecretKey:     secretKey,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileSecrets will reconcile all ArgoCD Secret resources.
func (r *ReconcileArgoCD) reconcileSecrets(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	err := r.reconcileCASecret(cr)
	if err != nil {
		return err
	}

	if err = r.reconcileArgoTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaSecret(cr); err != nil {
		return err
	}

	return nil
}
