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

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	tlsutil "github.com/operator-framework/operator-sdk/pkg/tls"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileArgoCD) getSecret(cr *argoproj.ArgoCD, name string) (secret *corev1.Secret, err error) {
	secret = newSecret(name, cr)
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name}, secret)
	return
}

// newCASecret creates a new CA secret for the given RethinkDBCluster.
func newCASecret(name string, cr *argoproj.ArgoCD) (*corev1.Secret, error) {
	secret := newTLSSecret(cr, name)

	key, err := newPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := newSelfSignedCACertificate(key)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       encodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: encodePrivateKeyPEM(key),
	}

	return secret, nil
}

// newCertificateSecret creates a new secret for a TLS certificate.
func newCertificateSecret(cr *argoproj.ArgoCD, name string, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*corev1.Secret, error) {
	secret := newTLSSecret(cr, name)

	key, err := newPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &tlsutil.CertConfig{
		CertName:     name,
		CertType:     tlsutil.ClientAndServingCert,
		CommonName:   name,
		Organization: []string{cr.ObjectMeta.Namespace},
	}

	dnsNames := []string{
		cr.ObjectMeta.Name,
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.ObjectMeta.Name, cr.ObjectMeta.Namespace),
	}

	cert, err := newSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       encodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: encodePrivateKeyPEM(key),
	}

	return secret, nil
}

// newSecret retuns a new Secret instance.
func newSecret(name string, cr *argoproj.ArgoCD) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

// newTLSSecret creates a new TLS secret with the given name for the given RethinkDBCluster.
func newTLSSecret(cr *argoproj.ArgoCD, name string) *corev1.Secret {
	secret := newSecret("argocd-tls-secret", cr)
	secret.ObjectMeta.Name = name
	secret.Type = corev1.SecretTypeTLS
	return secret
}

// reconcileArgoSecret will ensure that the ArgoCD Secret is present.
func (r *ReconcileArgoCD) reconcileArgoSecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-secret", cr)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileArgoTLSSecret ensures the TLS Secret is created for the ArgoCD Service.
func (r *ReconcileArgoCD) reconcileArgoTLSSecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-tls-secret", cr)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	caSecret, err := r.getSecret(cr, "argocd-ca-secret")
	if err != nil {
		return err
	}

	caCert, err := parsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := parsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = newCertificateSecret(cr, secret.Name, caCert, caKey)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}

	return r.client.Create(context.TODO(), secret)
}

// reconcileCASecret ensures the CA Secret is created.
func (r *ReconcileArgoCD) reconcileCASecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-ca-secret", cr)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	secret, err := newCASecret(secret.Name, cr)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ReconcileArgoCD) reconcileGrafanaSecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-grafana-secret", cr)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	secret.Data = map[string][]byte{
		"admin": []byte("secret"),
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

// reconcileSecrets will reconcile all ArgoCD Secret resources.
func (r *ReconcileArgoCD) reconcileSecrets(cr *argoproj.ArgoCD) error {
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

	if IsOpenShift() {
		if err := r.reconcileGrafanaSecret(cr); err != nil {
			return err
		}
	}

	return nil
}
