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
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-046_validate_application_tracking", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("verifies that when .spec.installationID is set, that value is set on Argo CD ConfigMap, and that installationID is also set on resources deployed by that Argo CD instance, and that .spec.resourceTrackingMethod is defined on that Argo CD instance", func() {

			By("creating namespaces which will contain Argo CD instances and which will be deployed to by Argo CD ")
			test_1_046_argocd_1_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-1-046-argocd-1")
			defer cleanupFunc()

			test_1_046_argocd_2_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-1-046-argocd-2")
			defer cleanupFunc()

			source_ns_1_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("source-ns-1")
			defer cleanupFunc()

			source_ns_2_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("source-ns-2")
			defer cleanupFunc()

			By("creating first Argo CD instance, with installationID 'instance-1', and annotation+label tracking")
			argocd_1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-1",
					Namespace: test_1_046_argocd_1_NS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					InstallationID:         "instance-1",
					ResourceTrackingMethod: "annotation+label",
				},
			}
			Expect(k8sClient.Create(ctx, argocd_1)).Should(Succeed())

			By("creating second Argo CD instance, with instance-2 ID, and annotation+label tracking")
			argocd_2 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-2",
					Namespace: test_1_046_argocd_2_NS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					InstallationID:         "instance-2",
					ResourceTrackingMethod: "annotation+label",
				},
			}
			Expect(k8sClient.Create(ctx, argocd_2)).Should(Succeed())

			Eventually(argocd_1, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argocd_2, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying argocd-cm for Argo CD instances contain the values defined in ArgoCD CR .spec field")
			configMap_test_1_046_argocd_1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: "test-1-046-argocd-1",
				},
			}
			Eventually(configMap_test_1_046_argocd_1).Should(k8sFixture.ExistByName())
			Expect(configMap_test_1_046_argocd_1).Should(configmapFixture.HaveStringDataKeyValue("installationID", "instance-1"))
			Expect(configMap_test_1_046_argocd_1).Should(configmapFixture.HaveStringDataKeyValue("application.resourceTrackingMethod", "annotation+label"))

			configMap_test_1_046_argocd_2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: "test-1-046-argocd-2",
				},
			}

			Eventually(configMap_test_1_046_argocd_2).Should(k8sFixture.ExistByName())
			Expect(configMap_test_1_046_argocd_2).Should(configmapFixture.HaveStringDataKeyValue("installationID", "instance-2"))
			Expect(configMap_test_1_046_argocd_2).Should(configmapFixture.HaveStringDataKeyValue("application.resourceTrackingMethod", "annotation+label"))

			By("adding managed-by label to test-1-046-argocd-(1/2), managed by Argo CD instances 1 and 2")
			namespace.Update(source_ns_1_NS, func(n *corev1.Namespace) {
				if n.Labels == nil {
					n.Labels = map[string]string{}
				}
				n.Labels["argocd.argoproj.io/managed-by"] = "test-1-046-argocd-1"
			})

			namespace.Update(source_ns_2_NS, func(n *corev1.Namespace) {
				if n.Labels == nil {
					n.Labels = map[string]string{}
				}
				n.Labels["argocd.argoproj.io/managed-by"] = "test-1-046-argocd-2"
			})

			By("verifying role is created in the correct source-ns-(1/2) namespaces, for instances")
			role_appController_source_ns_1 := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-1-argocd-application-controller",
					Namespace: "source-ns-1",
				},
			}
			Eventually(role_appController_source_ns_1).Should(k8sFixture.ExistByName())

			role_appController_source_ns_2 := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-2-argocd-application-controller",
					Namespace: "source-ns-2",
				},
			}
			Eventually(role_appController_source_ns_2).Should(k8sFixture.ExistByName())

			By("by defining a simple Argo CD Application for both Argo CD instances, to deploy to source namespaces 1/2 respectively")
			application_test_1_046_argocd_1 := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "test-1-046-argocd-1",
				},
				Spec: argocdv1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argocdv1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						Path:           "test/examples/nginx",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "source-ns-1",
					},
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, application_test_1_046_argocd_1)).To(Succeed())

			application_test_1_046_argocd_2 := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "test-1-046-argocd-2",
				},
				Spec: argocdv1alpha1.ApplicationSpec{
					Project: "default",
					Source: &argocdv1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						Path:           "test/examples/nginx",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "source-ns-2",
					},
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, application_test_1_046_argocd_2)).To(Succeed())

			By("verifying that the Applications successfully deployed, and that they have the correct installation-id and tracking-id, based on which Argo CD instance deployed them")

			Eventually(application_test_1_046_argocd_1, "4m", "5s").Should(application.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(application_test_1_046_argocd_1, "4m", "5s").Should(application.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

			Eventually(application_test_1_046_argocd_2, "4m", "5s").Should(application.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(application_test_1_046_argocd_2, "4m", "5s").Should(application.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

			deployment_source_ns_1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment",
					Namespace: "source-ns-1",
				},
			}
			Eventually(deployment_source_ns_1).Should(k8sFixture.ExistByName())
			Eventually(deployment_source_ns_1).Should(k8sFixture.HaveAnnotationWithValue("argocd.argoproj.io/installation-id", "instance-1"))
			Eventually(deployment_source_ns_1).Should(k8sFixture.HaveAnnotationWithValue("argocd.argoproj.io/tracking-id", "test-app:apps/Deployment:source-ns-1/nginx-deployment"))

			Eventually(deployment_source_ns_1).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/instance", "test-app"))

			deployment_source_ns_2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment",
					Namespace: "source-ns-2",
				},
			}
			Eventually(deployment_source_ns_2).Should(k8sFixture.ExistByName())
			Eventually(deployment_source_ns_2).Should(k8sFixture.HaveAnnotationWithValue("argocd.argoproj.io/installation-id", "instance-2"))
			Eventually(deployment_source_ns_2).Should(k8sFixture.HaveAnnotationWithValue("argocd.argoproj.io/tracking-id", "test-app:apps/Deployment:source-ns-2/nginx-deployment"))

			Eventually(deployment_source_ns_2).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/instance", "test-app"))

		})

	})
})
