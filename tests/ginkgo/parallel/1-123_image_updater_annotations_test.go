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
	"encoding/json"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	imageUpdaterApi "github.com/argoproj-labs/argocd-image-updater/api/v1alpha1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	applicationFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deplFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	ssFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-123_image_updater_annotations_test", func() {

		var (
			k8sClient    client.Client
			ctx          context.Context
			ns           *corev1.Namespace
			cleanupFunc  func()
			imageUpdater *imageUpdaterApi.ImageUpdater
			argoCD       *argov1beta1api.ArgoCD
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			// Cleanup is best-effort. Issue deletes and give some time for controllers
			// to process, but don't fail the test if cleanup takes too long.

			if imageUpdater != nil {
				By("deleting ImageUpdater CR")
				_ = k8sClient.Delete(ctx, imageUpdater)
			}

			if argoCD != nil {
				By("deleting ArgoCD CR")
				_ = k8sClient.Delete(ctx, argoCD)
			}

			if cleanupFunc != nil {
				cleanupFunc()
			}

			fixture.OutputDebugOnFail(ns)

		})

		It("ensures that Image Updater will update Argo CD Application using argocd (default) policy using legacy annotations", func() {

			By("creating simple namespace-scoped Argo CD instance with image updater enabled")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ImageUpdater: argov1beta1api.ArgoCDImageUpdaterSpec{
						Env: []corev1.EnvVar{
							{
								Name:  "IMAGE_UPDATER_LOGLEVEL",
								Value: "trace",
							},
							{
								Name:  "IMAGE_UPDATER_INTERVAL",
								Value: "0",
							},
						},
						Enabled: true},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "3s").Should(argocdFixture.BeAvailable())

			By("verifying all workloads are started")
			deploymentsShouldExist := []string{"argocd-redis", "argocd-server", "argocd-repo-server", "argocd-argocd-image-updater-controller"}
			for _, depl := range deploymentsShouldExist {
				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: depl, Namespace: ns.Name}}
				Eventually(depl).Should(k8sFixture.ExistByName())
				Eventually(depl).Should(deplFixture.HaveReplicas(1))
				Eventually(depl, "3m", "3s").Should(deplFixture.HaveReadyReplicas(1), depl.Name+" was not ready")
			}

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(statefulSet).Should(ssFixture.HaveReplicas(1))
			Eventually(statefulSet, "3m", "3s").Should(ssFixture.HaveReadyReplicas(1))

			By("creating Application")
			app := applicationFixture.Create("app-01", ns.Name,
				applicationFixture.WithRepo("https://github.com/argoproj-labs/argocd-image-updater/"),
				applicationFixture.WithPath("test/e2e/testdata/005-public-guestbook"),
				applicationFixture.WithRevision("HEAD"),
				applicationFixture.WithDestServer("https://kubernetes.default.svc"),
				applicationFixture.WithDestNamespace(ns.Name),
				applicationFixture.WithProject("default"),
				applicationFixture.WithAutoSync(),
				applicationFixture.WithAnnotation("argocd-image-updater.argoproj.io/image-list", "guestbook=quay.io/dkarpele/my-guestbook:~29437546.0"),
				applicationFixture.WithAnnotation("argocd-image-updater.argoproj.io/update-strategy", "semver"),
				applicationFixture.WithLabel("env", "prod"),
			)

			By("verifying deploying the Application succeeded")
			Eventually(app, "4m", "3s").Should(applicationFixture.HaveHealthStatus("Healthy"))
			Eventually(app, "4m", "3s").Should(applicationFixture.HaveSyncStatus("Synced"))

			By("creating ImageUpdater CR")
			useAnnotations := true
			imageUpdater = &imageUpdaterApi.ImageUpdater{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "image-updater",
					Namespace: ns.Name,
				},
				Spec: imageUpdaterApi.ImageUpdaterSpec{
					ApplicationRefs: []imageUpdaterApi.ApplicationRef{
						{
							NamePattern:    "app*",
							UseAnnotations: &useAnnotations,
							LabelSelectors: &metav1.LabelSelector{ // This is used for matching
								MatchLabels: map[string]string{"env": "prod"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, imageUpdater)).To(Succeed())

			By("ensuring that the Application image has `29437546.0` version after update")
			Eventually(func() string {
				// Use kubectl to get the Application JSON and extract the kustomize image
				// #nosec G204 -- test code
				cmd := exec.Command("kubectl", "get", "application.argoproj.io", app.Name,
					"-n", app.Namespace, "-o", "json")
				output, err := cmd.CombinedOutput()
				if err != nil {
					GinkgoWriter.Println(fmt.Sprintf("kubectl get application failed: %s", string(output)))
					return "" // Let Eventually retry on error
				}

				var appData map[string]interface{}
				if err := json.Unmarshal(output, &appData); err != nil {
					return ""
				}

				// Nil-safe traversal: spec.source.kustomize.images[0]
				spec, _ := appData["spec"].(map[string]interface{})
				if spec == nil {
					return ""
				}
				source, _ := spec["source"].(map[string]interface{})
				if source == nil {
					return ""
				}
				kustomize, _ := source["kustomize"].(map[string]interface{})
				if kustomize == nil {
					return ""
				}
				images, _ := kustomize["images"].([]interface{})
				if len(images) == 0 {
					return ""
				}
				img, _ := images[0].(string)
				return img
			}, "5m", "10s").Should(Equal("quay.io/dkarpele/my-guestbook:29437546.0"))
		})
	})
})
