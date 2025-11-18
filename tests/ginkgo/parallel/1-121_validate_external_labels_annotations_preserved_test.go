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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-121_validate_external_labels_annotations_preserved", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that labels set by external controllers are not deleted by reconciliation logic", func() {

			By("creating namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Labels: map[string]string{
							"operator-managed": "label",
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Labels: map[string]string{
							"operator-managed": "label",
						},
					},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Labels: map[string]string{
							"operator-managed": "label",
						},
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Labels: map[string]string{
							"operator-managed": "label",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for Argo CD to become available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			serverDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-server", Namespace: ns.Name}}
			repoDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-repo-server", Namespace: ns.Name}}
			appsetDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-applicationset-controller", Namespace: ns.Name}}
			controllerSS := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-sample-application-controller", Namespace: ns.Name}}

			By("verifying all components exist")
			Eventually(serverDepl).Should(k8sFixture.ExistByName())
			Eventually(repoDepl).Should(k8sFixture.ExistByName())
			Eventually(appsetDepl).Should(k8sFixture.ExistByName())
			Eventually(controllerSS).Should(k8sFixture.ExistByName())

			By("simulating external controller adding labels to server deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return err
				}
				if serverDepl.Spec.Template.Labels == nil {
					serverDepl.Spec.Template.Labels = make(map[string]string)
				}
				serverDepl.Spec.Template.Labels["external-controller-label"] = "external-value"
				serverDepl.Spec.Template.Labels["monitoring.io/enabled"] = "true"
				return k8sClient.Update(ctx, serverDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding labels to repo deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					return err
				}
				if repoDepl.Spec.Template.Labels == nil {
					repoDepl.Spec.Template.Labels = make(map[string]string)
				}
				repoDepl.Spec.Template.Labels["external-controller-label"] = "external-value"
				repoDepl.Spec.Template.Labels["backup.io/enabled"] = "true"
				return k8sClient.Update(ctx, repoDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding labels to applicationset deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return err
				}
				if appsetDepl.Spec.Template.Labels == nil {
					appsetDepl.Spec.Template.Labels = make(map[string]string)
				}
				appsetDepl.Spec.Template.Labels["external-controller-label"] = "external-value"
				appsetDepl.Spec.Template.Labels["sidecar.io/inject"] = "true"
				return k8sClient.Update(ctx, appsetDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding labels to controller statefulset")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					return err
				}
				if controllerSS.Spec.Template.Labels == nil {
					controllerSS.Spec.Template.Labels = make(map[string]string)
				}
				controllerSS.Spec.Template.Labels["external-controller-label"] = "external-value"
				controllerSS.Spec.Template.Labels["network-policy.io/allow"] = "true"
				return k8sClient.Update(ctx, controllerSS)
			}, "30s", "2s").Should(Succeed())

			By("verifying external labels were added successfully")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				return serverDepl.Spec.Template.Labels["external-controller-label"] == "external-value" &&
					serverDepl.Spec.Template.Labels["monitoring.io/enabled"] == "true"
			}, "30s", "2s").Should(BeTrue())

			By("triggering reconciliation by updating ArgoCD CR with a new label")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Labels["operator-managed-new"] = "new-label"
				ac.Spec.Repo.Labels["operator-managed-new"] = "new-label"
				ac.Spec.Controller.Labels["operator-managed-new"] = "new-label"
				ac.Spec.ApplicationSet.Labels["operator-managed-new"] = "new-label"
			})

			By("waiting for reconciliation to complete")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				return serverDepl.Spec.Template.Labels["operator-managed-new"] == "new-label"
			}, "2m", "5s").Should(BeTrue())

			By("verifying external labels on server deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					GinkgoWriter.Printf("Failed to get server deployment: %v\n", err)
					return false
				}
				labels := serverDepl.Spec.Template.Labels
				hasExternal := labels["external-controller-label"] == "external-value"
				hasMonitoring := labels["monitoring.io/enabled"] == "true"
				hasOperatorManaged := labels["operator-managed"] == "label"
				hasNewLabel := labels["operator-managed-new"] == "new-label"

				if !hasExternal || !hasMonitoring {
					GinkgoWriter.Printf("Server deployment missing external labels. Current labels: %v\n", labels)
					return false
				}
				if !hasOperatorManaged || !hasNewLabel {
					GinkgoWriter.Printf("Server deployment missing operator labels. Current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external labels on repo deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					GinkgoWriter.Printf("Failed to get repo deployment: %v\n", err)
					return false
				}
				labels := repoDepl.Spec.Template.Labels
				hasExternal := labels["external-controller-label"] == "external-value"
				hasBackup := labels["backup.io/enabled"] == "true"
				hasOperatorManaged := labels["operator-managed"] == "label"
				hasNewLabel := labels["operator-managed-new"] == "new-label"

				if !hasExternal || !hasBackup {
					GinkgoWriter.Printf("Repo deployment missing external labels. Current labels: %v\n", labels)
					return false
				}
				if !hasOperatorManaged || !hasNewLabel {
					GinkgoWriter.Printf("Repo deployment missing operator labels. Current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external labels on applicationset deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					GinkgoWriter.Printf("Failed to get appset deployment: %v\n", err)
					return false
				}
				labels := appsetDepl.Spec.Template.Labels
				hasExternal := labels["external-controller-label"] == "external-value"
				hasSidecar := labels["sidecar.io/inject"] == "true"
				hasOperatorManaged := labels["operator-managed"] == "label"
				hasNewLabel := labels["operator-managed-new"] == "new-label"

				if !hasExternal || !hasSidecar {
					GinkgoWriter.Printf("Applicationset deployment missing external labels. Current labels: %v\n", labels)
					return false
				}
				if !hasOperatorManaged || !hasNewLabel {
					GinkgoWriter.Printf("Applicationset deployment missing operator labels. Current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external labels on controller statefulset are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					GinkgoWriter.Printf("Failed to get controller statefulset: %v\n", err)
					return false
				}
				labels := controllerSS.Spec.Template.Labels
				hasExternal := labels["external-controller-label"] == "external-value"
				hasNetworkPolicy := labels["network-policy.io/allow"] == "true"
				hasOperatorManaged := labels["operator-managed"] == "label"
				hasNewLabel := labels["operator-managed-new"] == "new-label"

				if !hasExternal || !hasNetworkPolicy {
					GinkgoWriter.Printf("Controller statefulset missing external labels. Current labels: %v\n", labels)
					return false
				}
				if !hasOperatorManaged || !hasNewLabel {
					GinkgoWriter.Printf("Controller statefulset missing operator labels. Current labels: %v\n", labels)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("simulating external controller adding annotations to server deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return err
				}
				if serverDepl.Spec.Template.Annotations == nil {
					serverDepl.Spec.Template.Annotations = make(map[string]string)
				}
				serverDepl.Spec.Template.Annotations["external-controller-annotation"] = "external-annotation-value"
				return k8sClient.Update(ctx, serverDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding annotations to repo deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					return err
				}
				if repoDepl.Spec.Template.Annotations == nil {
					repoDepl.Spec.Template.Annotations = make(map[string]string)
				}
				repoDepl.Spec.Template.Annotations["external-controller-annotation"] = "external-annotation-value"
				return k8sClient.Update(ctx, repoDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding annotations to applicationset deployment")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return err
				}
				if appsetDepl.Spec.Template.Annotations == nil {
					appsetDepl.Spec.Template.Annotations = make(map[string]string)
				}
				appsetDepl.Spec.Template.Annotations["external-controller-annotation"] = "external-annotation-value"
				return k8sClient.Update(ctx, appsetDepl)
			}, "30s", "2s").Should(Succeed())

			By("simulating external controller adding annotations to controller statefulset")
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					return err
				}
				if controllerSS.Spec.Template.Annotations == nil {
					controllerSS.Spec.Template.Annotations = make(map[string]string)
				}
				controllerSS.Spec.Template.Annotations["external-controller-annotation"] = "external-annotation-value"
				return k8sClient.Update(ctx, controllerSS)
			}, "30s", "2s").Should(Succeed())

			By("verifying external annotations were added successfully")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				return serverDepl.Spec.Template.Annotations["external-controller-annotation"] == "external-annotation-value"
			}, "30s", "2s").Should(BeTrue())

			By("triggering reconciliation by adding operator-managed annotations")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Annotations = map[string]string{
					"operator-annotation": "operator-value",
				}
				ac.Spec.Repo.Annotations = map[string]string{
					"operator-annotation": "operator-value",
				}
				ac.Spec.Controller.Annotations = map[string]string{
					"operator-annotation": "operator-value",
				}
				ac.Spec.ApplicationSet.Annotations = map[string]string{
					"operator-annotation": "operator-value",
				}
			})

			By("waiting for operator annotations to be applied")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				return serverDepl.Spec.Template.Annotations["operator-annotation"] == "operator-value"
			}, "2m", "5s").Should(BeTrue())

			By("verifying external annotations on server deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					GinkgoWriter.Printf("Failed to get server deployment: %v\n", err)
					return false
				}
				annotations := serverDepl.Spec.Template.Annotations
				hasExternal := annotations["external-controller-annotation"] == "external-annotation-value"
				hasOperator := annotations["operator-annotation"] == "operator-value"

				if !hasExternal {
					GinkgoWriter.Printf("Server deployment missing external annotations. Current annotations: %v\n", annotations)
					return false
				}
				if !hasOperator {
					GinkgoWriter.Printf("Server deployment missing operator annotation. Current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external annotations on repo deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					GinkgoWriter.Printf("Failed to get repo deployment: %v\n", err)
					return false
				}
				annotations := repoDepl.Spec.Template.Annotations
				hasExternal := annotations["external-controller-annotation"] == "external-annotation-value"
				hasOperator := annotations["operator-annotation"] == "operator-value"

				if !hasExternal {
					GinkgoWriter.Printf("Repo deployment missing external annotations. Current annotations: %v\n", annotations)
					return false
				}
				if !hasOperator {
					GinkgoWriter.Printf("Repo deployment missing operator annotation. Current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external annotations on applicationset deployment are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					GinkgoWriter.Printf("Failed to get appset deployment: %v\n", err)
					return false
				}
				annotations := appsetDepl.Spec.Template.Annotations
				hasExternal := annotations["external-controller-annotation"] == "external-annotation-value"
				hasOperator := annotations["operator-annotation"] == "operator-value"

				if !hasExternal {
					GinkgoWriter.Printf("Applicationset deployment missing external annotations. Current annotations: %v\n", annotations)
					return false
				}
				if !hasOperator {
					GinkgoWriter.Printf("Applicationset deployment missing operator annotation. Current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("verifying external annotations on controller statefulset are preserved after reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					GinkgoWriter.Printf("Failed to get controller statefulset: %v\n", err)
					return false
				}
				annotations := controllerSS.Spec.Template.Annotations
				hasExternal := annotations["external-controller-annotation"] == "external-annotation-value"
				hasOperator := annotations["operator-annotation"] == "operator-value"

				if !hasExternal {
					GinkgoWriter.Printf("Controller statefulset missing external annotations. Current annotations: %v\n", annotations)
					return false
				}
				if !hasOperator {
					GinkgoWriter.Printf("Controller statefulset missing operator annotation. Current annotations: %v\n", annotations)
					return false
				}
				return true
			}, "2m", "5s").Should(BeTrue())

			By("updating operator-managed annotations to trigger another reconciliation")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Annotations["operator-annotation-new"] = "new-operator-value"
				ac.Spec.Repo.Annotations["operator-annotation-new"] = "new-operator-value"
				ac.Spec.Controller.Annotations["operator-annotation-new"] = "new-operator-value"
				ac.Spec.ApplicationSet.Annotations["operator-annotation-new"] = "new-operator-value"
			})

			By("verifying external annotations and labels are still preserved after second reconciliation")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					return false
				}
				labels := serverDepl.Spec.Template.Labels
				annotations := serverDepl.Spec.Template.Annotations
				return labels["external-controller-label"] == "external-value" &&
					annotations["external-controller-annotation"] == "external-annotation-value" &&
					annotations["operator-annotation-new"] == "new-operator-value"
			}, "2m", "5s").Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDepl), repoDepl); err != nil {
					return false
				}
				labels := repoDepl.Spec.Template.Labels
				annotations := repoDepl.Spec.Template.Annotations
				return labels["external-controller-label"] == "external-value" &&
					annotations["external-controller-annotation"] == "external-annotation-value" &&
					annotations["operator-annotation-new"] == "new-operator-value"
			}, "2m", "5s").Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl); err != nil {
					return false
				}
				labels := appsetDepl.Spec.Template.Labels
				annotations := appsetDepl.Spec.Template.Annotations
				return labels["external-controller-label"] == "external-value" &&
					annotations["external-controller-annotation"] == "external-annotation-value" &&
					annotations["operator-annotation-new"] == "new-operator-value"
			}, "2m", "5s").Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSS), controllerSS); err != nil {
					return false
				}
				labels := controllerSS.Spec.Template.Labels
				annotations := controllerSS.Spec.Template.Annotations
				return labels["external-controller-label"] == "external-value" &&
					annotations["external-controller-annotation"] == "external-annotation-value" &&
					annotations["operator-annotation-new"] == "new-operator-value"
			}, "2m", "5s").Should(BeTrue())
		})

	})
})
