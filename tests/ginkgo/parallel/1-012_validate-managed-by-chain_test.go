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

package parallel

import (
	"context"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerManagerName      = "argocd-operator-controller-manager"
	controllerManagerNamespace = "argocd-operator-system"
)

func ensureNamespaceManagementEnabledForTest(ctx context.Context, k8sClient client.Client) (cleanup func()) {
	operatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerManagerName,
			Namespace: controllerManagerNamespace,
		},
	}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(operatorDeployment), operatorDeployment); err != nil {
		By("no in-cluster controller-manager Deployment — skipping " + common.EnableManagedNamespace + " patch; test still runs (set env on the operator process when running locally)")
		return func() {}
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

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-012_validate-managed-by-chain", func() {

		var (
			ctx                     context.Context
			k8sClient               client.Client
			cleanupfuncs            []func()
			nsTest_1_12_custom      *corev1.Namespace
			nsTest_1_12_custom2     *corev1.Namespace
			randomNS                *corev1.Namespace
			nsTest_1_12_nm_argocd   *corev1.Namespace
			nsTest_1_12_nm_tenant   *corev1.Namespace
			argoCDWithNamespaceMgmt *argov1beta1api.ArgoCD
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
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
			for _, ns := range []*corev1.Namespace{nsTest_1_12_custom, nsTest_1_12_custom2, randomNS, nsTest_1_12_nm_argocd, nsTest_1_12_nm_tenant} {
				if ns != nil {
					debugParams = append(debugParams, ns)
				}
			}
			fixture.OutputDebugOnFail(debugParams...)
		})

		It("validates that namespace-scoped Argo CD instance is able to managed other namespaces, including when those namespaces are deleted", func() {

			By("creating ArgoCD instance and 2 custom namespaces ")

			var cleanupFunc1, cleanupFunc2, cleanupFunc3, cleanupFunc4 func()

			nsTest_1_12_custom, cleanupFunc1 = fixture.CreateNamespaceWithCleanupFunc("test-1-12-custom")
			cleanupfuncs = append(cleanupfuncs, cleanupFunc1)

			nsTest_1_12_custom2, cleanupFunc2 = fixture.CreateNamespaceWithCleanupFunc("test-1-12-custom2")
			cleanupfuncs = append(cleanupfuncs, cleanupFunc2)

			randomNS, cleanupFunc3 = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			cleanupfuncs = append(cleanupfuncs, cleanupFunc3)

			argoCDRandomNS := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: randomNS.Name},
			}
			Expect(k8sClient.Create(ctx, argoCDRandomNS)).To(Succeed())

			Eventually(argoCDRandomNS, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("configuring test-1-12-custom to be managed by Argo CD instance")

			k8sFixture.Update(nsTest_1_12_custom, func(obj client.Object) {
				nsObj, ok := obj.(*corev1.Namespace)
				Expect(ok).To(BeTrue())
				if nsObj.Labels == nil {
					nsObj.Labels = map[string]string{}
				}
				nsObj.Labels["argocd.argoproj.io/managed-by"] = argoCDRandomNS.Namespace
			})

			// Verify Role/RoleBinding for the managed namespace is validate
			expectRoleAndRoleBindingAreValidForManagedNamespace := func(managedNS string) {
				Eventually(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-server", Namespace: managedNS}}).Should(k8sFixture.ExistByName())

				Eventually(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-application-controller", Namespace: managedNS}}).Should(k8sFixture.ExistByName())

				rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-server", Namespace: managedNS}}
				Eventually(rb).Should(k8sFixture.ExistByName())

				Expect(rb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
				Expect(rb.RoleRef.Kind).To(Equal("Role"))
				Expect(rb.RoleRef.Name).To(Equal("argocd-argocd-server"))
				Expect(rb.Subjects).To(HaveLen(1))
				Expect(rb.Subjects[0]).To(Equal(rbacv1.Subject{Kind: "ServiceAccount", Name: "argocd-argocd-server", Namespace: argoCDRandomNS.Namespace}))
			}
			expectRoleAndRoleBindingAreValidForManagedNamespace(nsTest_1_12_custom.Name)

			By("verifying 'argocd-default-cluster-config' cluster secret references both argo cd ns and custom ns 1")
			clusterSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-default-cluster-config", Namespace: argoCDRandomNS.Namespace},
			}
			Eventually(clusterSecret).Should(secretFixture.HaveStringDataKeyValue("namespaces",
				argoCDRandomNS.Namespace+","+nsTest_1_12_custom.Name))

			By("creating Argo CD Application targeting test-1-12-custom")
			app := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1-12-custom", Namespace: argoCDRandomNS.Namespace},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/nginx",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: nsTest_1_12_custom.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: "default",
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("verifying Argo CD is successfully able to reconcile and deploy the resources of the test Argo CD Application, into test-1-12-custom")

			Eventually(app, "4m", "5s").Should(appFixture.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(app, "4m", "5s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

			By("setting test-1-12-custom2 to be managed by Argo CD instance")
			k8sFixture.Update(nsTest_1_12_custom2, func(obj client.Object) {
				nsObj, ok := obj.(*corev1.Namespace)
				Expect(ok).To(BeTrue())
				if nsObj.Labels == nil {
					nsObj.Labels = map[string]string{}
				}
				nsObj.Labels["argocd.argoproj.io/managed-by"] = argoCDRandomNS.Namespace

			})

			By("validating role/rolebindings are valid for second managed namespace")
			expectRoleAndRoleBindingAreValidForManagedNamespace(nsTest_1_12_custom2.Name)

			By("validating Argo CD is able to deploy to second managed namespace")
			app2 := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1-12-custom2", Namespace: argoCDRandomNS.Namespace},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/nginx",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: nsTest_1_12_custom2.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: "default",
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app2)).To(Succeed())

			Eventually(app2, "4m", "1s").Should(appFixture.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(app2, "4m", "1s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

			By("deleting all Argo CD applications and first managed namespace")

			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			Expect(k8sClient.Delete(ctx, app2)).To(Succeed())

			Expect(k8sClient.Delete(ctx, nsTest_1_12_custom)).To(Succeed())

			By("verifying 'argocd-default-cluster-config' cluster secret references both argo cd ns and custom ns 2 (but not custom ns 1, since it was deleted)")
			clusterSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-default-cluster-config", Namespace: argoCDRandomNS.Namespace},
			}
			Eventually(clusterSecret).Should(secretFixture.HaveStringDataKeyValue("namespaces",
				argoCDRandomNS.Namespace+","+nsTest_1_12_custom2.Name))

			By("recreating Argo CD applications")

			nsTest_1_12_custom, cleanupFunc4 = fixture.CreateNamespaceWithCleanupFunc("test-1-12-custom")
			cleanupfuncs = append(cleanupfuncs, cleanupFunc4)

			app = &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1-12-custom", Namespace: argoCDRandomNS.Namespace},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/nginx",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: nsTest_1_12_custom.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: "default",
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
						Retry: &argocdv1alpha1.RetryStrategy{
							Limit: 5,
						},
					},
				},
			}
			app2 = &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1-12-custom2", Namespace: argoCDRandomNS.Namespace},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/nginx",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: nsTest_1_12_custom2.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: "default",
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
						Retry: &argocdv1alpha1.RetryStrategy{
							Limit: 5,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			Expect(k8sClient.Create(ctx, app2)).To(Succeed())

			By("verifying Argo CD can deploy to managed NS 2, but can no longer deploy to managed NS 1")

			Eventually(app, "1m", "1s").Should(appFixture.HaveHealthStatusCode(health.HealthStatusMissing))
			Eventually(app, "1m", "1s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeUnknown))

			Eventually(app2, "4m", "1s").Should(appFixture.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(app2, "4m", "1s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

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
			argoCDWithNamespaceMgmt = &argov1beta1api.ArgoCD{
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
