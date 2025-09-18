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

			By("partially removing some custom labels from server and repo deployment (selective removal)")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Labels = map[string]string{
					"custom2": "server",
				}
				ac.Spec.Repo.Labels = map[string]string{
					"custom": "label",
				}
			})

			By("verifying selective label removal from server deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				labels := serverDepl.Spec.Template.Labels
				if _, exists := labels["custom"]; exists {
					GinkgoWriter.Printf("Label 'custom' still exists in server deployment, current labels: %v\n", labels)
					return false
				}

				// Should still have these labels
				if labels["custom2"] != "server" {
					GinkgoWriter.Printf("Label 'custom2' missing or incorrect in server deployment, current labels: %v\n", labels)
					return false
				}

				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying selective label removal from repo deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					return false
				}
				labels := repoDepl.Spec.Template.Labels
				if _, exists := labels["custom2"]; exists {
					GinkgoWriter.Printf("Label 'custom2' still exists in repo deployment, current labels: %v\n", labels)
					return false
				}

				// Should still have these labels
				if labels["custom"] != "label" {
					GinkgoWriter.Printf("Label 'custom' missing or incorrect in repo deployment, current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying that the labels of the other components are not affected")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return false
				}
				//Above operation should not affect the labels of the other components
				appsetLabels := appsetDepl.Spec.Template.Labels
				_, hasCustom := appsetLabels["custom"]
				_, hasCustom2 := appsetLabels["custom2"]

				if !hasCustom || !hasCustom2 {
					GinkgoWriter.Printf("Label 'custom' or 'custom2' missing from repo deployment, current labels: %v\n", appsetLabels)
					return false
				}

				controllerLabels := appsetDepl.Spec.Template.Labels
				_, hasCustom = controllerLabels["custom"]
				_, hasCustom2 = controllerLabels["custom2"]

				if !hasCustom || !hasCustom2 {
					GinkgoWriter.Printf("Label 'custom' or 'custom2' missing from repo deployment, current labels: %v\n", controllerLabels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("partially removing some custom labels from controller and appset deployment (selective removal)")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Labels = map[string]string{
					"custom": "label",
				}
				ac.Spec.ApplicationSet.Labels = map[string]string{
					"custom2": "applicationSet",
				}
			})

			By("verifying selective label removal from controller deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					return false
				}
				labels := controllerSS.Spec.Template.Labels
				if _, exists := labels["custom2"]; exists {
					GinkgoWriter.Printf("Label 'custom2' still exists in controller deployment, current labels: %v\n", labels)
					return false
				}
				// Should still have these labels
				if labels["custom"] != "label" {
					GinkgoWriter.Printf("Label 'custom' missing or incorrect in controller deployment, current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying selective label removal from appset deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return false
				}
				labels := appsetDepl.Spec.Template.Labels
				if _, exists := labels["custom"]; exists {
					GinkgoWriter.Printf("Label 'custom' still exists in appset deployment, current labels: %v\n", labels)
					return false
				}
				// Should still have these labels
				if labels["custom2"] != "applicationSet" {
					GinkgoWriter.Printf("Label 'custom2' missing or incorrect in appset deployment, current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying that the labels of server and repo deployments are not affected")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return false
				}
				_, hasCustom := serverDepl.Spec.Template.Labels["custom2"]
				if !hasCustom {
					GinkgoWriter.Printf("Label 'custom2' missing from server deployment, current labels: %v\n", serverDepl.Spec.Template.Labels)
					return false
				}
				_, hasCustom = repoDepl.Spec.Template.Labels["custom"]
				if !hasCustom {
					GinkgoWriter.Printf("Label 'custom' missing from repo deployment, current labels: %v\n", repoDepl.Spec.Template.Labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("partially removing some custom annotations from controller and server deployment (selective removal)")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Annotations = map[string]string{
					"custom": "annotation",
				}
				ac.Spec.Server.Annotations = map[string]string{
					"custom2": "server",
				}
			})

			By("verifying selective annotation removal from controller deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					return false
				}
				annotations := controllerSS.Spec.Template.Annotations
				if _, exists := annotations["custom2"]; exists {
					GinkgoWriter.Printf("Annotation 'custom2' still exists in controller deployment, current annotations: %v\n", annotations)
					return false
				}
				// Should still have these annotations
				if annotations["custom"] != "annotation" {
					GinkgoWriter.Printf("Annotation 'custom' missing or incorrect in controller deployment, current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying selective annotation removal from server deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				annotations := serverDepl.Spec.Template.Annotations
				if _, exists := annotations["custom"]; exists {
					GinkgoWriter.Printf("Annotation 'custom' still exists in server deployment, current annotations: %v\n", annotations)
					return false
				}
				// Should still have these annotations
				if annotations["custom2"] != "server" {
					GinkgoWriter.Printf("Annotation 'custom2' missing or incorrect in server deployment, current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("partially removing some custom annotations from repo and appset deployment (selective removal)")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Repo.Annotations = map[string]string{
					"custom2": "repo",
				}
				ac.Spec.ApplicationSet.Annotations = map[string]string{
					"custom": "annotation",
				}
			})

			By("verifying selective annotation removal from repo deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					return false
				}
				annotations := repoDepl.Spec.Template.Annotations
				if _, exists := annotations["custom"]; exists {
					GinkgoWriter.Printf("Annotation 'custom' still exists in repo deployment, current annotations: %v\n", annotations)
					return false
				}
				// Should still have these annotations
				if annotations["custom2"] != "repo" {
					GinkgoWriter.Printf("Annotation 'custom2' missing or incorrect in repo deployment, current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying selective annotation removal from appset deployment")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return false
				}
				annotations := appsetDepl.Spec.Template.Annotations
				if _, exists := annotations["custom2"]; exists {
					GinkgoWriter.Printf("Annotation 'custom2' still exists in appset deployment, current annotations: %v\n", annotations)
					return false
				}
				// Should still have these annotations
				if annotations["custom"] != "annotation" {
					GinkgoWriter.Printf("Annotation 'custom' missing or incorrect in appset deployment, current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("completelyremoving all custom labels and annotations from ArgoCD CR")

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

			Eventually(serverDepl).Should(k8sFixture.ExistByName())
			Eventually(controllerSS).Should(k8sFixture.ExistByName())
			Eventually(appsetDepl).Should(k8sFixture.ExistByName())
			Eventually(repoDepl).Should(k8sFixture.ExistByName())

			expectLabelsAndAnnotationsRemovedFromDepl := func(depl *appsv1.Deployment, componentName string) {

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
					// Verify operator-managed labels are preserved
					if _, exists := depl.Spec.Template.Labels["app.kubernetes.io/name"]; !exists {
						GinkgoWriter.Printf("Operator-managed label 'app.kubernetes.io/name' missing from %s deployment, current labels: %v\n", componentName, depl.Spec.Template.Labels)
						return false
					}

					return true
				}, "2m", "5s").Should(BeTrue())

			}

			expectLabelsAndAnnotationsRemovedFromDepl(appsetDepl, "applicationset")
			expectLabelsAndAnnotationsRemovedFromDepl(serverDepl, "server")
			expectLabelsAndAnnotationsRemovedFromDepl(repoDepl, "repo")

			// Evaluate the controller statefulset on its own, since it's a StatefulSet not a Deployment
			Eventually(controllerSS).Should(k8sFixture.ExistByName())

			for k := range controllerSS.Spec.Template.Annotations {
				Expect(k).ToNot(ContainSubstring("custom"))
			}

			for k := range controllerSS.Spec.Template.Labels {
				Expect(k).ToNot(ContainSubstring("custom"))
			}

			By("adding more labels")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Labels = map[string]string{
					"second-label": "second-value",
					"third-label":  "third-value",
				}
			})

			By("verifying new labels are added")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				labels := serverDepl.Spec.Template.Labels
				return labels["second-label"] == "second-value" &&
					labels["third-label"] == "third-value"
			}, "2m", "5s").Should(BeTrue())
		})

	})
})
