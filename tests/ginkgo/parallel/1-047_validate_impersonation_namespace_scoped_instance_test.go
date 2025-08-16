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

	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
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

	Context("1-047_validate_impersonation_namespace_scoped_instance", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("creates a namespace-scoped Argo CD instance, then verifies that when impersonation is enabled, that Argo CD is not able to deploy to a Namespace which the appproject does not have access to", func() {

			By("creating simple namespace-scoped Argo CD instance with impersonation enabled")
			argoCD_NS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
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
			guestbookNS, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("guestbook-1-047", argoCD.Namespace)
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
			appProject := &appv1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-proj",
					Namespace: argoCD_NS.Name,
				},
				Spec: appv1alpha1.AppProjectSpec{
					ClusterResourceWhitelist: []metav1.GroupKind{{
						Group: "*",
						Kind:  "*",
					}},
					DestinationServiceAccounts: []appv1alpha1.ApplicationDestinationServiceAccount{{
						DefaultServiceAccount: serviceAccount.Name,
						Namespace:             guestbookNS.Name,
						Server:                "https://kubernetes.default.svc",
					}},
					Destinations: []appv1alpha1.ApplicationDestination{{
						Namespace: guestbookNS.Name,
						Server:    "https://kubernetes.default.svc",
					}},
					SourceRepos: []string{"https://github.com/argoproj/argocd-example-apps.git"},
				},
			}
			Expect(k8sClient.Create(ctx, appProject)).Should(Succeed())

			By("creating an Application which deploys to guestbook namespace, which should succeed to deploy")
			guestbookApplication := &appv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook",
					Namespace: argoCD.Namespace,
				},
				Spec: appv1alpha1.ApplicationSpec{
					Destination: appv1alpha1.ApplicationDestination{
						Namespace: guestbookNS.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: appProject.Name,
					Source: &appv1alpha1.ApplicationSource{
						Directory: &appv1alpha1.ApplicationSourceDirectory{
							Jsonnet: appv1alpha1.ApplicationSourceJsonnet{},
							Recurse: true,
						},
						Path:    "guestbook",
						RepoURL: "https://github.com/argoproj/argocd-example-apps",
					},
					SyncPolicy: &appv1alpha1.SyncPolicy{
						Automated: &appv1alpha1.SyncPolicyAutomated{},
						SyncOptions: []string{
							"ServerSideApply=true",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, guestbookApplication)).To(Succeed())
			Eventually(guestbookApplication, "2m", "5s").Should(applicationFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))

			By("creating a new namespace 'guestbook-dev' which is managed by our argo cd instance")
			guestbookDevNS, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("guestbook-dev-1-047", argoCD.Namespace)
			defer cleanupFunc()

			By("waiting for managed-by roles to be created in guestbook-dev namespace")
			appControllerRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-argocd-application-controller",
					Namespace: guestbookDevNS.Name,
				},
			}
			Eventually(appControllerRole, "4m", "5s").Should(k8sFixture.ExistByName())

			By("updating AppProject destinations to allow deployment to new namespace, but we DON'T add a new serviceaccount within that namespace, as we did previously")
			appprojectFixture.Update(appProject, func(ap *appv1alpha1.AppProject) {
				appProject.Spec.Destinations = append(appProject.Spec.Destinations, appv1alpha1.ApplicationDestination{
					Namespace: guestbookDevNS.Name,
					Server:    "https://kubernetes.default.svc",
				})
			})

			By("creating a new Application that attempts to deploy to that new namespace")
			guestbookDevApplication := &appv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-dev",
					Namespace: argoCD.Namespace,
				},
				Spec: appv1alpha1.ApplicationSpec{
					Destination: appv1alpha1.ApplicationDestination{
						Namespace: guestbookDevNS.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: appProject.Name,
					Source: &appv1alpha1.ApplicationSource{
						Directory: &appv1alpha1.ApplicationSourceDirectory{
							Jsonnet: appv1alpha1.ApplicationSourceJsonnet{},
							Recurse: true,
						},
						Path:    "guestbook",
						RepoURL: "https://github.com/argoproj/argocd-example-apps",
					},
					SyncPolicy: &appv1alpha1.SyncPolicy{
						Automated: &appv1alpha1.SyncPolicyAutomated{},
						SyncOptions: []string{
							"ServerSideApply=true",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, guestbookDevApplication)).Should(Succeed())

			By("verifying Argo CD is not able to deploy to that new namespace, because impersonation prevents it, since there is no matching service account defined in AppProject for Argo CD to use")
			Eventually(guestbookDevApplication).Should(applicationFixture.HaveHealthStatusCode(health.HealthStatusMissing))

			By("verifying ServiceAccount never existed in namespace")
			guestbook_dev_ServiceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook-deployer",
					Namespace: guestbookDevNS.Name,
				},
			}
			Consistently(guestbook_dev_ServiceAccount).ShouldNot(k8sFixture.ExistByName())

			By("verifying Application contains error message indicating that no matching service account exists in the appproject, which is required for impersonation")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(guestbookDevApplication), guestbookDevApplication)).To(Succeed())

			Eventually(guestbookDevApplication, "4m", "5s").Should(applicationFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeOutOfSync))

			Expect(guestbookDevApplication.Status.OperationState).ToNot(BeNil())

			Expect(guestbookDevApplication.Status.OperationState.Message).To(ContainSubstring("failed to find a matching service account to impersonate: no matching service account found for destination server"))

		})
	})
})
