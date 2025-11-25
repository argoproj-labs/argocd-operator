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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-043_validate_custom_labels_annotations", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that custom labels and annotations set on component fields of ArgoCD CR will be added to Deployment and StatefulSet templates of those components, and that they can likewise be removed", func() {

			By("creating namespace-scoped Argo CD instance with labels and annotations set on components")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Labels: map[string]string{
							"custom":  "label",
							"custom2": "server",
						},
						Annotations: map[string]string{
							"custom":  "annotation",
							"custom2": "server",
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Labels: map[string]string{
							"custom":  "label",
							"custom2": "repo",
						},
						Annotations: map[string]string{
							"custom":  "annotation",
							"custom2": "repo",
						},
					},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Labels: map[string]string{
							"custom":  "label",
							"custom2": "controller",
						},
						Annotations: map[string]string{
							"custom":  "annotation",
							"custom2": "controller",
						},
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Labels: map[string]string{
							"custom":  "label",
							"custom2": "applicationSet",
						},
						Annotations: map[string]string{
							"custom":  "annotation",
							"custom2": "applicationSet",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for Argo CD to become available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Argo CD components have the labels and annotations we set above")

			serverDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-server", Namespace: ns.Name}}

			Eventually(serverDepl).Should(k8sFixture.ExistByName())
			Expect(serverDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("app.kubernetes.io/name", "argocd-sample-server"))
			Expect(serverDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom", "label"))
			Expect(serverDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom2", "server"))

			Expect(serverDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom", "annotation"))
			Expect(serverDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom2", "server"))

			repoDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-repo-server", Namespace: ns.Name}}
			Eventually(repoDepl).Should(k8sFixture.ExistByName())
			Expect(repoDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("app.kubernetes.io/name", "argocd-sample-repo-server"))
			Expect(repoDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom", "label"))
			Expect(repoDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom2", "repo"))
			Expect(repoDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom", "annotation"))
			Expect(repoDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom2", "repo"))

			appsetDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-applicationset-controller", Namespace: ns.Name}}
			Eventually(appsetDepl).Should(k8sFixture.ExistByName())
			Expect(appsetDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("app.kubernetes.io/name", "argocd-sample-applicationset-controller"))
			Expect(appsetDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom", "label"))
			Expect(appsetDepl).Should(deploymentFixture.HaveTemplateLabelWithValue("custom2", "applicationSet"))
			Expect(appsetDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom", "annotation"))
			Expect(appsetDepl).Should(deploymentFixture.HaveTemplateAnnotationWithValue("custom2", "applicationSet"))

			controllerSS := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-application-controller", Namespace: ns.Name}}
			Eventually(controllerSS).Should(k8sFixture.ExistByName())
			Expect(controllerSS).Should(statefulsetFixture.HaveTemplateLabelWithValue("app.kubernetes.io/name", "argocd-sample-application-controller"))
			Expect(controllerSS).Should(statefulsetFixture.HaveTemplateLabelWithValue("custom", "label"))
			Expect(controllerSS).Should(statefulsetFixture.HaveTemplateLabelWithValue("custom2", "controller"))
			Expect(controllerSS).Should(statefulsetFixture.HaveTemplateAnnotationWithValue("custom", "annotation"))
			Expect(controllerSS).Should(statefulsetFixture.HaveTemplateAnnotationWithValue("custom2", "controller"))

			By("removing custom labels and annotations from ArgoCD CR")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Labels = map[string]string{}
				ac.Spec.Server.Annotations = map[string]string{}

				ac.Spec.Repo.Labels = map[string]string{}
				ac.Spec.Repo.Annotations = map[string]string{}

				ac.Spec.Controller.Labels = map[string]string{}
				ac.Spec.Controller.Annotations = map[string]string{}

				ac.Spec.ApplicationSet.Labels = map[string]string{}
				ac.Spec.ApplicationSet.Annotations = map[string]string{}
			})

			By("verifying labels and annotations have been removed from template specs of Argo CD components")

			Eventually(controllerSS).Should(k8sFixture.ExistByName())
			Eventually(appsetDepl).Should(k8sFixture.ExistByName())
			Eventually(repoDepl).Should(k8sFixture.ExistByName())

			expectLabelsAndAnnotationsRemovedFromDepl := func(depl *appsv1.Deployment) {

				By("checking labels and annotations are removed from " + depl.Name)

				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl); err != nil {
						GinkgoWriter.Println(err)
						return false
					}

					for k := range depl.Spec.Template.Annotations {
						if strings.Contains(k, "custom") {
							return false
						}
					}

					for k := range depl.Spec.Template.Labels {
						if strings.Contains(k, "custom") {
							return false
						}
					}

					return true
				}).Should(BeTrue())

			}

			expectLabelsAndAnnotationsRemovedFromDepl(appsetDepl)
			expectLabelsAndAnnotationsRemovedFromDepl(serverDepl)
			expectLabelsAndAnnotationsRemovedFromDepl(repoDepl)

			// Evaluate the controller statefulset on its own, since it's a StatefulSet not a Deployment
			Eventually(controllerSS).Should(k8sFixture.ExistByName())

			for k := range controllerSS.Spec.Template.Annotations {
				Expect(k).ToNot(ContainSubstring("custom"))
			}

			for k := range controllerSS.Spec.Template.Labels {
				Expect(k).ToNot(ContainSubstring("custom"))
			}

		})
	})
})
