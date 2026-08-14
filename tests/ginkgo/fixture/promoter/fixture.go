// promoterFixture provides helpful functions for the GitOps Promoter E2E tests
package promoterFixture

import (
	"context"
	"crypto/x509"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/certutil"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

const caSubject = "gitops-promoter-ca"

type PromoterAPIServerTLSSecretConfig struct {
	PromoterNamespace       string
	APIServerCertSecretName string
	APIServerServiceName    string
	CABundleSecretName      string
	CABundleSecretKey       string
}

type VerifyExpectedResourcesExistParams struct {
	Namespace               *corev1.Namespace
	Deployment              *appsv1.Deployment
	Service                 *corev1.Service
	ControllerConfiguration *promoter.ControllerConfiguration
	ClusterRoleNames        []string
	ClusterRoleBindingNames []string
	RoleBinding             *rbacv1.RoleBinding
	ServiceAccount          *corev1.ServiceAccount
	APIService              *apiregistrationv1.APIService
}

func CreateAPIServerTLSSecrets(cfg PromoterAPIServerTLSSecretConfig) {
	k8sClient, _ := utils.GetE2ETestKubeClient()
	ctx := context.Background()

	By("Creating API Server TLS secrets")

	caKey, caCert, caCertPEM := certutil.GenerateCertificateAuthority(caSubject)
	caBundleSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.CABundleSecretName,
			Namespace: cfg.PromoterNamespace,
		},
		Data: map[string][]byte{
			cfg.CABundleSecretKey: caCertPEM,
		},
	}
	Expect(k8sClient.Create(ctx, caBundleSecret)).To(Succeed())

	apiServerCertPEM, apiServerKeyPEM := certutil.IssueCertificate(caCert, caKey, certutil.CertificateRequest{
		CommonName: cfg.APIServerServiceName,
		DNSNames: []string{
			cfg.APIServerServiceName,
			fmt.Sprintf("%s.%s", cfg.APIServerServiceName, cfg.PromoterNamespace),
			fmt.Sprintf("%s.%s.svc", cfg.APIServerServiceName, cfg.PromoterNamespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", cfg.APIServerServiceName, cfg.PromoterNamespace),
		},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	apiServerTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.APIServerCertSecretName,
			Namespace: cfg.PromoterNamespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": apiServerCertPEM,
			"tls.key": apiServerKeyPEM,
			"ca.crt":  caCertPEM,
		},
	}
	Expect(k8sClient.Create(ctx, apiServerTLSSecret)).To(Succeed())
}

func VerifyExpectedResourcesExist(resources VerifyExpectedResourcesExistParams) {
	By("Verifying the expected resources exist")

	if resources.Deployment != nil {
		Eventually(resources.Deployment).Should(k8sFixture.ExistByName())
	}

	if resources.Service != nil {
		Eventually(resources.Service).Should(k8sFixture.ExistByName())
	}

	if resources.ControllerConfiguration != nil {
		Eventually(resources.ControllerConfiguration).Should(k8sFixture.ExistByName())
	}

	if resources.RoleBinding != nil {
		Eventually(resources.RoleBinding).Should(k8sFixture.ExistByName())
	}

	if resources.ServiceAccount != nil {
		Eventually(resources.ServiceAccount).Should(k8sFixture.ExistByName())
	}

	for _, clusterRoleName := range resources.ClusterRoleNames {
		if clusterRoleName == "" {
			continue
		}
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleName,
			},
		}
		Eventually(clusterRole).Should(k8sFixture.ExistByName())
	}

	for _, clusterRoleBindingName := range resources.ClusterRoleBindingNames {
		if clusterRoleBindingName == "" {
			continue
		}
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
			},
		}
		Eventually(clusterRoleBinding).Should(k8sFixture.ExistByName())
	}

	if resources.APIService != nil {
		Eventually(resources.APIService).Should(k8sFixture.ExistByName())
	}
}

func VerifyExpectedResourcesDontExist(resources VerifyExpectedResourcesExistParams) {
	By("Verifying the expected resources don't exist")

	if resources.Deployment != nil {
		Eventually(resources.Deployment).Should(k8sFixture.NotExistByName())
	}

	if resources.Service != nil {
		Eventually(resources.Service).Should(k8sFixture.NotExistByName())
	}

	if resources.ControllerConfiguration != nil {
		Eventually(resources.ControllerConfiguration).Should(k8sFixture.NotExistByName())
	}

	if resources.RoleBinding != nil {
		Eventually(resources.RoleBinding).Should(k8sFixture.NotExistByName())
	}

	if resources.ServiceAccount != nil {
		Eventually(resources.ServiceAccount).Should(k8sFixture.NotExistByName())
	}

	for _, clusterRoleName := range resources.ClusterRoleNames {
		if clusterRoleName == "" {
			continue
		}
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleName,
			},
		}
		Eventually(clusterRole).Should(k8sFixture.NotExistByName())
	}

	for _, clusterRoleBindingName := range resources.ClusterRoleBindingNames {
		if clusterRoleBindingName == "" {
			continue
		}
		clusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterRoleBindingName,
			},
		}
		Eventually(clusterRoleBinding).Should(k8sFixture.NotExistByName())
	}

	if resources.APIService != nil {
		Eventually(resources.APIService).Should(k8sFixture.NotExistByName())
	}
}
