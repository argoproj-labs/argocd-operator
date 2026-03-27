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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	applicationFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	appprojectFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/appproject"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-046_validate_impersonation_cluster_scoped_instance", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("creates a cluster-scoped Argo CD instance, then verifies that when impersonation is enabled, that Argo CD is not able to deploy to a Namespace which the appproject does not have access to", func() {

			By("creating simple cluster-scoped Argo CD instance. The namespace should be specified within ARGOCD_CLUSTER_CONFIG_NAMESPACES env var, in order to ensure it is cluster-scoped")
			argoCD_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-test-impersonation-1-046")
			defer cleanupFunc()

			By("creating ArgoCD instance with application.sync.impersonation.enabled")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: argoCD_NS.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ExtraConfig: map[string]string{
						"application.sync.impersonation.enabled": "true",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying ConfigMap contains impersonation value specified in ArgoCD CR")
			argocdCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: argoCD_NS.Name,
				},
			}
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("application.sync.impersonation.enabled", "true"))

			By("creating guestbook namespace which we will deploy to")

			guestbookNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("guestbook-1-046")
			defer cleanupFunc()

			By("creating ServiceAccount in guestbook NS that Argo CD will deploy with")
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-deployer",
					Namespace: guestbookNS.Name,
				},
			}
			Expect(k8sClient.Create(ctx, serviceAccount)).Should(Succeed())

			By("creating RoleBinding in guestbook NS for ServiceAccount")
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-deployer-rb",
					Namespace: guestbookNS.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     "cluster-admin",
				},
				Subjects: []rbacv1.Subject{{
					Kind:      "ServiceAccount",
					Name:      serviceAccount.Name,
					Namespace: guestbookNS.Name,
				}},
			}
			Expect(k8sClient.Create(ctx, roleBinding)).To(Succeed())

			By("creating AppProject which allows us to deploy to guestbook namespace using a specific ServiceAccount")
			projRef := appprojectFixture.Create("guestbook-proj", argoCD_NS.Name,
				appprojectFixture.WithSourceRepo("https://github.com/argoproj/argocd-example-apps.git"),
				appprojectFixture.WithDestination("https://kubernetes.default.svc", guestbookNS.Name),
				appprojectFixture.WithClusterResource("*", "*"),
				appprojectFixture.WithDestinationServiceAccount("https://kubernetes.default.svc", guestbookNS.Name, serviceAccount.Name),
			)

			By("creating an Application which deploys to guestbook namespace, which should succeed to deploy")
			guestbookRef := applicationFixture.Create("guestbook", argoCD.Namespace,
				applicationFixture.WithRepo("https://github.com/argoproj/argocd-example-apps"),
				applicationFixture.WithPath("guestbook"),
				applicationFixture.WithDestServer("https://kubernetes.default.svc"),
				applicationFixture.WithDestNamespace(guestbookNS.Name),
				applicationFixture.WithProject("guestbook-proj"),
				applicationFixture.WithAutoSync(),
				applicationFixture.WithDirectoryRecurse(),
				applicationFixture.WithSyncOption("ServerSideApply=true"),
				applicationFixture.WithSyncOption("CreateNamespace=true"),
			)
			Eventually(guestbookRef, "4m", "5s").Should(applicationFixture.HaveSyncStatus("Synced"))

			By("updating AppProject to allow deployment to a different namespace, but we DON'T add a new serviceaccount within that namespace, as we did previously")
			appprojectFixture.AddDestination(projRef, "https://kubernetes.default.svc", "guestbook-dev-1-046")

			By("creating a new Application that attempts to deploy to that new namespace")
			guestbookDevRef := applicationFixture.Create("guestbook-dev", argoCD.Namespace,
				applicationFixture.WithRepo("https://github.com/argoproj/argocd-example-apps"),
				applicationFixture.WithPath("guestbook"),
				applicationFixture.WithDestServer("https://kubernetes.default.svc"),
				applicationFixture.WithDestNamespace("guestbook-dev-1-046"),
				applicationFixture.WithProject("guestbook-proj"),
				applicationFixture.WithAutoSync(),
				applicationFixture.WithDirectoryRecurse(),
				applicationFixture.WithSyncOption("ServerSideApply=true"),
			)

			By("verifying Argo CD is not able to deploy to that new namespace, because impersonation prevents it, since there is no matching service account defined in AppProject for Argo CD to use")
			Eventually(guestbookDevRef).Should(applicationFixture.HaveHealthStatus("Missing"))

			By("verifying ServiceAccount never existed in namespace (which doesnt exist)")
			guestbook_dev_ServiceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-deployer",
					Namespace: "guestbook-dev-1-046",
				},
			}
			Consistently(guestbook_dev_ServiceAccount).ShouldNot(k8sFixture.ExistByName())

			By("verifying Application contains error message indicating that no matching service account exists in the appproject, which is required for impersonation")
			msg, err := applicationFixture.GetOperationMessage(guestbookDevRef)
			Expect(err).ToNot(HaveOccurred())
			Expect(msg).To(ContainSubstring("failed to find a matching service account to impersonate: no matching service account found for destination server"))

		})
	})
})
