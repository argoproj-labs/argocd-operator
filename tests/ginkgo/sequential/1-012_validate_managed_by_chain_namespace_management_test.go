/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sequential

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerManagerNameForNamespaceManagementTest      = "argocd-operator-controller-manager"
	controllerManagerNamespaceForNamespaceManagementTest = "argocd-operator-system"
)

func ensureNamespaceManagementEnabledForTest(ctx context.Context, k8sClient client.Client) (cleanup func()) {
	operatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerManagerNameForNamespaceManagementTest,
			Namespace: controllerManagerNamespaceForNamespaceManagementTest,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(operatorDeployment), operatorDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			By("no in-cluster controller-manager Deployment - skipping " + common.EnableManagedNamespace + " patch; test still runs (set env on the operator process when running locally)")
			return func() {}
		}
		Expect(err).NotTo(HaveOccurred(), "failed to read controller-manager Deployment")
	}
	By("enabling NamespaceManagement feature")
	originalEnvValue, _ := deploymentFixture.GetEnv(operatorDeployment, "manager", common.EnableManagedNamespace)
	deploymentFixture.SetEnv(operatorDeployment, "manager", common.EnableManagedNamespace, "true")
	Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

	return func() {
		By("restoring operator EnableManagedNamespace env var")
		if originalEnvValue != nil {
			deploymentFixture.SetEnv(operatorDeployment, "manager", common.EnableManagedNamespace, *originalEnvValue)
		} else {
			deploymentFixture.RemoveEnv(operatorDeployment, "manager", common.EnableManagedNamespace)
		}
		Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))
	}
}

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {
	Context("1-012_validate_managed_by_chain_namespace_management", func() {
		var (
			ctx                   context.Context
			k8sClient             client.Client
			cleanupfuncs          []func()
			nsTest_1_12_nm_argocd *corev1.Namespace
			nsTest_1_12_nm_tenant *corev1.Namespace
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			cleanupfuncs = make([]func(), 0)
		})

		AfterEach(func() {
			defer func() {
				for _, cleanupfunc := range cleanupfuncs {
					cleanupfunc()
				}
			}()
			var debugParams []any
			for _, ns := range []*corev1.Namespace{nsTest_1_12_nm_argocd, nsTest_1_12_nm_tenant} {
				if ns != nil {
					debugParams = append(debugParams, ns)
				}
			}
			fixture.OutputDebugOnFail(debugParams...)
		})

		It("validates that with spec.namespaceManagement and a NamespaceManagement CR the operator applies the managed-by label and RBAC to the tenant namespace so Applications are discovered (fix for issue #2039)", func() {
			cleanup := ensureNamespaceManagementEnabledForTest(ctx, k8sClient)
			defer cleanup()

			By("creating namespace for Argo CD instance")
			nsTest_1_12_nm_argocd, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("test-1-12-nm-argocd")
			cleanupfuncs = append(cleanupfuncs, cleanupFunc1)

			By("creating tenant namespace to be managed via NamespaceManagement")
			nsTest_1_12_nm_tenant, cleanupFunc2 := fixture.CreateNamespaceWithCleanupFunc("test-1-12-nm-tenant")
			cleanupfuncs = append(cleanupfuncs, cleanupFunc2)

			By("creating namespace-scoped Argo CD with spec.namespaceManagement allowing the tenant namespace")
			argoCDWithNamespaceMgmt := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: nsTest_1_12_nm_argocd.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					NamespaceManagement: []argov1beta1api.ManagedNamespaces{
						{
							Name:           nsTest_1_12_nm_tenant.Name,
							AllowManagedBy: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDWithNamespaceMgmt)).To(Succeed())

			By("creating NamespaceManagement CR in tenant namespace with managedBy set to Argo CD namespace")
			nm := &argov1beta1api.NamespaceManagement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tenant-nm",
					Namespace: nsTest_1_12_nm_tenant.Name,
				},
				Spec: argov1beta1api.NamespaceManagementSpec{
					ManagedBy: nsTest_1_12_nm_argocd.Name,
				},
			}
			Expect(k8sClient.Create(ctx, nm)).To(Succeed())

			By("waiting for Argo CD instance to become available")
			Eventually(argoCDWithNamespaceMgmt, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying tenant namespace gets argocd.argoproj.io/managed-by label")
			Eventually(nsTest_1_12_nm_tenant, "2m", "5s").Should(namespaceFixture.HaveLabel(common.ArgoCDManagedByLabel, nsTest_1_12_nm_argocd.Name))
			Consistently(nsTest_1_12_nm_tenant, "10s", "2s").Should(namespaceFixture.HaveLabel(common.ArgoCDManagedByLabel, nsTest_1_12_nm_argocd.Name))

			By("verifying RBAC is created in tenant namespace")
			Eventually(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-server", Namespace: nsTest_1_12_nm_tenant.Name}}).Should(k8sFixture.ExistByName())
			Eventually(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-application-controller", Namespace: nsTest_1_12_nm_tenant.Name}}).Should(k8sFixture.ExistByName())
			rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-server", Namespace: nsTest_1_12_nm_tenant.Name}}
			Eventually(rb).Should(k8sFixture.ExistByName())
			Expect(rb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
			Expect(rb.RoleRef.Kind).To(Equal("Role"))
			Expect(rb.RoleRef.Name).To(Equal("argocd-argocd-server"))
			Expect(rb.Subjects).To(HaveLen(1))
			Expect(rb.Subjects[0]).To(Equal(rbacv1.Subject{Kind: "ServiceAccount", Name: "argocd-argocd-server", Namespace: nsTest_1_12_nm_argocd.Name}))
		})
	})
})
