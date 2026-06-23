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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-082_validate_node_placement", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that setting values in .spec.nodePlacement on Argo CD CR cause those values to be set on the Argo CD Deployment/StatefulSet workloads", func() {

			By("creating a basic Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("setting ArgoCD .spec.nodePlacement field with 'key1': 'value1'. The operator should set the value on all Argo CD workloads")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.NodePlacement = &argov1beta1api.ArgoCDNodePlacementSpec{
					NodeSelector: map[string]string{
						"key1": "value1",
					},
				}
			})

			By("verifying that Argo CD Deployments use the nodeSelector value from ArgoCD CR")
			deploymentNameList := []string{"example-argocd-redis", "example-argocd-repo-server", "example-argocd-server"}

			for _, deploymentName := range deploymentNameList {

				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name}}
				Expect(depl).To(k8sFixture.ExistByName())

				Eventually(depl).Should(deploymentFixture.HaveTemplateSpecNodeSelector(map[string]string{"key1": "value1", "kubernetes.io/os": "linux"}))

			}

			By("verifying that Argo CD StatefulSet uses the nodeSelector value from ArgoCD CR")
			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-application-controller", Namespace: ns.Name}}
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(statefulSet).Should(statefulsetFixture.HaveTemplateSpecNodeSelector(map[string]string{"key1": "value1", "kubernetes.io/os": "linux"}))

			By("adding a toleration to the nodePlacement in the Argo CD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.NodePlacement = &argov1beta1api.ArgoCDNodePlacementSpec{
					NodeSelector: map[string]string{
						"key1": "value1",
					},
					Tolerations: []corev1.Toleration{
						{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule},
					},
				}
			})

			By("verifying the Argo CD Deployments are updated to include the toleration change in nodePlacement of CR")
			for _, deploymentName := range deploymentNameList {

				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name}}
				Expect(depl).To(k8sFixture.ExistByName())

				Eventually(depl).Should(deploymentFixture.HaveTemplateSpecNodeSelector(map[string]string{"key1": "value1", "kubernetes.io/os": "linux"}))
				Eventually(depl).Should(deploymentFixture.HaveTolerations([]corev1.Toleration{{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule}}))

			}

			By("verifying the Argo CD StatefulSet is updated to include the toleration change in nodePlacement of CR")
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(statefulSet).Should(statefulsetFixture.HaveTemplateSpecNodeSelector(map[string]string{"key1": "value1", "kubernetes.io/os": "linux"}))

			Eventually(statefulSet).Should(statefulsetFixture.HaveTolerations([]corev1.Toleration{{Key: "key1", Operator: corev1.TolerationOpEqual, Value: "value1", Effect: corev1.TaintEffectNoSchedule}}))

		})

	})
})
