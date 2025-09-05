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
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-002_validate_cluster_config", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {

			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that cluster roles and role bindings are created for cluster-scoped ArgoCD instance, and that values set in initialSSHKnownHosts will be set in ConfigMap", func() {

			By("creating simple cluster-scoped Argo CD instance with initialSSHKnownHosts set")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					InitialSSHKnownHosts: argov1beta1api.SSHHostsSpec{
						ExcludeDefaultHosts: true,
						Keys:                "github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

			By("verifying ClusterRole/Bindings exist")
			appcontrollerCRB := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-e2e-cluster-config-argocd-application-controller"},
			}
			Eventually(appcontrollerCRB).Should(k8sFixture.ExistByName())

			serverCRB := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-e2e-cluster-config-argocd-server"},
			}
			Eventually(serverCRB).Should(k8sFixture.ExistByName())

			appControllerCR := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-argocd-e2e-cluster-config-argocd-application-controller",
				},
			}
			Eventually(appControllerCR).Should(k8sFixture.ExistByName())

			serverCR := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-argocd-e2e-cluster-config-argocd-server",
				},
			}
			Eventually(serverCR).Should(k8sFixture.ExistByName())

			By("verifying ConfigMap 'argocd-ssh-known-hosts-cm' has the value from InitialSSHKnownHosts")

			knownHostsCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-ssh-known-hosts-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(knownHostsCM).Should(k8sFixture.ExistByName())
			Eventually(knownHostsCM).Should(configmap.HaveStringDataKeyValue("ssh_known_hosts", "github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ=="))

			By("enabling applicationset and setting sourceNamespaces and SCMProviders")

			argocdFixture.Update(argoCDInstance, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet = &argov1beta1api.ArgoCDApplicationSet{
					SourceNamespaces: []string{"some-namespace", "some-other-namespace"},
					SCMProviders:     []string{"github.com"},
				}
			})

			By("verifying ClusterRole/Bindings exist")

			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			appSetClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-e2e-cluster-config-argocd-applicationset-controller"},
			}
			Eventually(appSetClusterRole).Should(k8sFixture.ExistByName())

			appSetCRB := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-argocd-e2e-cluster-config-argocd-applicationset-controller",
				},
			}
			Eventually(appSetCRB).Should(k8sFixture.ExistByName())

		})
	})
})
