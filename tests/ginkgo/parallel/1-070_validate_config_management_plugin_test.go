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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	applicationFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-070_validate_config_management_plugin_test", func() {
		// This test supersedes 1-017_validate_cmp

		var (
			k8sClient   client.Client
			ctx         context.Context
			ns          *corev1.Namespace
			cleanupFunc func()
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			defer cleanupFunc()
			if ns != nil {
				fixture.OutputDebugOnFail(ns.Name)
			}
		})

		It("validates that an Argo CD Application with a ConfigManagementPlugin mounted via sidecar will deploy as expected", func() {

			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			By("creating simple namespace-scoped Argo CD instance with a CMP sidecar, where the sidecar is a simple bash script")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SidecarContainers: []corev1.Container{{
							Name:    "cmp",
							Command: []string{"/var/run/argocd/argocd-cmp-server"}, // Entrypoint should be Argo CD lightweight CMP server ie. argocd-cmp-server
							Image:   "quay.io/fedora/fedora:latest",                // This can be off-the-shelf or custom-built image
							SecurityContext: &corev1.SecurityContext{
								RunAsNonRoot: ptr.To(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{MountPath: "/var/run/argocd", Name: "var-files"},
								{MountPath: "/home/argocd/cmp-server/plugins", Name: "plugins"},
								{MountPath: "/tmp", Name: "tmp"},
								// Remove this volumeMount if you've chosen to bake the config file into the sidecar image.
								{MountPath: "/home/argocd/cmp-server/config/plugin.yaml", Name: "cmp-plugin", SubPath: "plugin.yaml"},
							},
						}},
						Volumes: []corev1.Volume{{Name: "cmp-plugin", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cmp-plugin"}}}}},
					},
				},
			}

			if !fixture.RunningOnOpenShift() {
				argoCD.Spec.Repo.SidecarContainers[0].SecurityContext.RunAsUser = ptr.To(int64(999))
			}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("creating ConfigMap with CMP plugin that will echo a ConfigMap that does not exist in source repository")
			cmpPluginCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cmp-plugin", Namespace: ns.Name},
				Data: map[string]string{"plugin.yaml": `apiVersion: v1
apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: cmp-plugin
spec:
  version: v1.0
  generate:
    command: [sh, -c, 'echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"Bar\": \"baz\"}}}"']
  discover:
    find:
      command: [sh, -c, 'echo "FOUND"; exit 0']
  allowConcurrency: true
  lockRepo: true`},
			}

			Expect(k8sClient.Create(ctx, cmpPluginCM)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("creating an Argo CD Application which will deploy guestbook example using CMP plugin")
			app := &appv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook",
					Namespace: ns.Name,
				},
				Spec: appv1alpha1.ApplicationSpec{
					Project: "default",
					Source: &appv1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						Path:           "test/examples/guestbook",
						TargetRevision: "HEAD",
						Plugin: &appv1alpha1.ApplicationSourcePlugin{
							Env: appv1alpha1.Env{{Name: "FOO", Value: "myfoo"}},
						},
					},
					Destination: appv1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: ns.Name,
					},
					SyncPolicy: &appv1alpha1.SyncPolicy{Automated: &appv1alpha1.SyncPolicyAutomated{}},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("verifying deploying the Application succeeded")
			Eventually(app, "4m", "5s").Should(applicationFixture.HaveHealthStatusCode(health.HealthStatusHealthy))
			Eventually(app, "4m", "5s").Should(applicationFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))

			By("verifying that the ConfigMap generated by the CMP plugin was successfully deployed to the target namespace")
			guestbookCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "guestbook", Namespace: ns.Name}}
			Eventually(guestbookCM).Should(k8sFixture.ExistByName())
		})

	})
})
