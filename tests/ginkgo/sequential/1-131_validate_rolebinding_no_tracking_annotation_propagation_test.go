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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

// When an ArgoCD CR is itself managed by another Argo CD instance, it
// carries Argo CD resource-tracking annotations (e.g. argocd.argoproj.io/tracking-id). The
// operator should not propagate those annotations onto the RoleBindings it creates in managed
// namespaces, otherwise the central Argo CD treats those RoleBindings as owned resources and
// prunes them.
var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-131_validate_rolebinding_no_tracking_annotation_propagation", func() {

		const (
			trackingIDAnnotation     = "argocd.argoproj.io/tracking-id"
			installationIDAnnotation = "argocd.argoproj.io/installation-id"
		)

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("does not propagate Argo CD tracking annotations from the ArgoCD CR to RoleBindings created in managed namespaces", func() {

			By("creating a namespace to contain the Argo CD instance")
			argoCDNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appteam-argocd-1-131")
			defer cleanupFunc()

			By("creating a managed namespace where the operator will create RoleBindings")
			managedNS, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("appteam-apps-1-131", argoCDNS.Name)
			defer cleanupFunc()

			By("creating an ArgoCD instance carrying tracking annotations, as if deployed by a central Argo CD")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appteam",
					Namespace: argoCDNS.Name,
					Annotations: map[string]string{
						trackingIDAnnotation:     "central-gitops:argoproj.io/ArgoCD:central/appteam",
						installationIDAnnotation: "central-argocd",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			rbNames := []string{"appteam-argocd-application-controller", "appteam-argocd-server"}

			By("verifying the operator created the expected RoleBindings in the managed namespace")
			for _, rbName := range rbNames {
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: managedNS.Name},
				}
				Eventually(roleBinding, "60s", "5s").Should(k8sFixture.ExistByName())
			}

			By("verifying the RoleBindings did NOT inherit the Argo CD tracking annotations from the CR")
			for _, rbName := range rbNames {
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: managedNS.Name},
				}
				// The operator's own default annotation should be present
				Eventually(roleBinding).Should(k8sFixture.HaveAnnotationWithValue(common.AnnotationName, argoCD.Name))
				// Central Argo CD's tracking annotations are not present.
				Consistently(roleBinding, "15s", "3s").Should(k8sFixture.NotHaveAnnotation(trackingIDAnnotation))
				Consistently(roleBinding, "5s", "1s").Should(k8sFixture.NotHaveAnnotation(installationIDAnnotation))
			}

			By("verifying the RoleBindings in the Argo CD's own namespace also did NOT inherit the tracking annotations")
			for _, rbName := range rbNames {
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: argoCDNS.Name},
				}
				Eventually(roleBinding, "60s", "5s").Should(k8sFixture.ExistByName())
				Eventually(roleBinding).Should(k8sFixture.HaveAnnotationWithValue(common.AnnotationName, argoCD.Name))
				Consistently(roleBinding, "5s", "1s").Should(k8sFixture.NotHaveAnnotation(trackingIDAnnotation))
				Consistently(roleBinding, "5s", "1s").Should(k8sFixture.NotHaveAnnotation(installationIDAnnotation))
			}
		})

	})
})
