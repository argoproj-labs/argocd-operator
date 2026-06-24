package parallel

import (
	"context"
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/gitserver"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-126_source_hydrator", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
			ns        *corev1.Namespace
			cleanup   func()
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(ns)
			cleanup()
		})

		It("activate commit server by Source Hydrator config", func() {
			ns, cleanup = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			csService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "example-commit-server", Namespace: ns.Name}}
			csNetPol := &v1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-commit-server-network-policy", Namespace: ns.Name}}
			csSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-commit-server", Namespace: ns.Name}}

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: ns.Name,
				},
				// The existence of CommitServer section does NOT start the commit server - only when actual use-cases are detected.
				Spec: argov1beta1api.ArgoCDSpec{
					CommitServer: argov1beta1api.ArgoCDCommitServerSpec{
						LogLevel: "debug",
					},
				},
			}

			assertRunning := func(running bool) {
				exist := k8sFixture.ExistByName()

				if running {
					haveStatus := argocdFixture.HaveCommitServerStatus("Running")
					Eventually(argoCD, "30s", "5s").Should(haveStatus)
					Consistently(argoCD, "10s", "2s").Should(haveStatus)
					Expect(csService).Should(exist)
					Expect(csNetPol).Should(exist)
					Expect(csSA).Should(exist)
				} else {
					haveStatus := argocdFixture.HaveCommitServerStatus("")
					Eventually(argoCD, "30s", "5s").Should(haveStatus)
					Consistently(argoCD, "10s", "2s").Should(haveStatus)
					Expect(csService).ShouldNot(exist)
					Expect(csNetPol).ShouldNot(exist)
					Expect(csSA).ShouldNot(exist)
				}
			}

			By("Not running by default")
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			assertRunning(false)

			By("Running when enabled")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.SourceHydrator.Enabled = ptr.To(true)
			})
			assertRunning(true)

			By("Not running when disabled")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.SourceHydrator.Enabled = ptr.To(false)
			})
			assertRunning(false)
		})

		It("apply CommitServer configuration options to the running CommitServer deployment", func() {
			ns, cleanup = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			expectedCommand := []string{"/usr/local/bin/argocd-commit-server", "--loglevel", "info", "--logformat", "json"}
			deploymentObjectKey := client.ObjectKey{Namespace: ns.Name, Name: "cs-options-commit-server"}
			fetchDeployment := func() *appsv1.Deployment {
				deploy := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, deploymentObjectKey, deploy)).Should(Succeed())
				return deploy
			}

			By("creating ArgoCD with detailed CommitServer spec")

			logLevel := "info"
			logFormat := "json"
			annotations := map[string]string{
				"example-annotation": "commit-server-test",
			}
			labels := map[string]string{
				"example-label": "commit-server-test",
			}
			envVars := []corev1.EnvVar{
				{Name: "FOO", Value: "BAR"},
				{Name: "LOG_FEATURE", Value: "enabled"},
			}
			resources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			}
			initContainers := []corev1.Container{
				{
					Name:            "init-commit-server",
					Image:           common.ArgoCDDefaultArgoImage,
					ImagePullPolicy: corev1.PullAlways,
					Command:         []string{"echo", "init"},
				},
			}

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cs-options",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceHydrator: argov1beta1api.ArgoCDSourceHydratorSpec{
						Enabled: ptr.To(true),
					},
					CommitServer: argov1beta1api.ArgoCDCommitServerSpec{
						LogLevel:       logLevel,
						LogFormat:      logFormat,
						Annotations:    annotations,
						Labels:         labels,
						Env:            envVars,
						Resources:      &resources,
						InitContainers: initContainers,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("Attributes configured innitially")
			deploy := fetchDeployment()

			Expect(deploy.Spec.Template.Annotations).To(HaveKeyWithValue("example-annotation", "commit-server-test"))
			Expect(deploy.Spec.Template.Labels).To(HaveKeyWithValue("example-label", "commit-server-test"))

			container := deploy.Spec.Template.Spec.Containers[0]
			Expect(container.Command).To(Equal(expectedCommand))
			for _, expectedEnv := range envVars {
				Expect(container.Env).To(ContainElement(expectedEnv))
			}
			Expect(container.Resources).To(Equal(resources))

			Expect(deploy.Spec.Template.Spec.InitContainers).To(HaveLen(1))
			ic := deploy.Spec.Template.Spec.InitContainers[0]
			Expect(ic.Name).To(Equal("init-commit-server"))
			Expect(ic.Image).To(Equal(common.ArgoCDDefaultArgoImage))
			Expect(ic.Command).To(Equal([]string{"echo", "init"}))

			By("Annotations reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.Annotations = nil
			})
			Eventually(func() map[string]string {
				return fetchDeployment().Spec.Template.Annotations
			}, "5s", "1s").ToNot(HaveKey("example-annotation"))

			By("Labels reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.Labels = nil
			})
			Eventually(func() map[string]string {
				return fetchDeployment().Spec.Template.Labels
			}, "5s", "1s").ToNot(HaveKey("example-label"))

			By("Env reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.Env = nil
			})
			Eventually(func() []corev1.EnvVar {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Env
			}, "5s", "1s").ToNot(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))

			By("Resources reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.Resources.Limits = nil
			})
			Eventually(func() corev1.ResourceRequirements {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Resources
			}, "5s", "1s").To(Equal(corev1.ResourceRequirements{Requests: resources.Requests})) // Same Requests, no Limits

			By("InitContainers reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.InitContainers[0].Command = []string{"echo", "init-2"}
			})
			Eventually(func() []string {
				return fetchDeployment().Spec.Template.Spec.InitContainers[0].Command
			}, "5s", "1s").To(Equal([]string{"echo", "init-2"}))

			By("Command reconciles")
			argocdFixture.Update(argoCD, func(argoCD *argov1beta1api.ArgoCD) {
				argoCD.Spec.CommitServer.LogLevel = "debug"
			})
			Eventually(func() []string {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Command
			}, "5s", "1s").To(ContainElement("debug"))
		})

		It("hydrate kustomize to another branch via ssh", func() {
			ns, cleanup = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			server, gitCleanup := gitserver.StartServer(ctx, k8sClient, ns)
			defer gitCleanup()
			knownHosts := server.SSHKnownHosts()
			Expect(knownHosts).NotTo(BeEmpty())

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-hydrator",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					InitialSSHKnownHosts: argov1beta1api.SSHHostsSpec{
						ExcludeDefaultHosts: true,
						Keys:                knownHosts,
					},
					SourceHydrator: argov1beta1api.ArgoCDSourceHydratorSpec{
						Enabled: ptr.To(true),
					},
					// Expose host for the webhook to work.
					Server: argov1beta1api.ArgoCDServerSpec{
						Ingress: argov1beta1api.ArgoCDIngressSpec{
							Enabled: true,
						},
						Insecure: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Expect(argoCD.Status.Host).NotTo(BeEmpty())

			repo := server.CreateRepo("hydrator-kustomize")

			By("creating a test Argo CD Application")
			app := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "hydrated", Namespace: ns.Name},
				Spec: argocdv1alpha1.ApplicationSpec{
					Project: "default",
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: ns.Name,
						Server:    "https://kubernetes.default.svc",
					},

					SourceHydrator: &argocdv1alpha1.SourceHydrator{
						DrySource: argocdv1alpha1.DrySource{
							Path:           "app/overlays/prod",
							RepoURL:        repo.GetRepoSshURL(),
							TargetRevision: "dry",
						},
						SyncSource: argocdv1alpha1.SyncSource{
							Path:         "app",
							TargetBranch: "hydrated",
						},
					},
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{
							Prune:    ptr.To(true),
							SelfHeal: ptr.To(true),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("pushing dry source to trigger Source Hydrator")
			Expect(repo.Clone()).To(Succeed())
			Expect(repo.CommitAndPush(gitserver.Commit{
				Branch:            "dry",
				NotifyWebhookURL:  gitserver.ArgoCDWebhookURL(argoCD.Status.Host, argoCD.Spec.Server.Insecure),
				NotifyWebhookHost: argoCD.Name,
				Files: map[string]string{
					"app/base/kustomization.yaml": `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- configmap.yaml
`,
					"app/base/configmap.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: source-hydrator-test
data:
  foo: base
`,
					"app/overlays/prod/kustomization.yaml": `
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../base
patches:
- target:
    kind: ConfigMap
    name: source-hydrator-test
  patch: |-
    - op: replace
      path: /data/foo
      value: prod
`,
				},
			})).To(Succeed())

			By("waiting for Source Hydrator to hydrate and sync the application")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), app)).To(Succeed())
				g.Expect(app.Status.SourceHydrator.CurrentOperation).NotTo(BeNil())
				g.Expect(app.Status.SourceHydrator.CurrentOperation.Phase).To(
					Equal(argocdv1alpha1.HydrateOperationPhaseHydrated),
				)
			}, "1m", "5s").Should(Succeed())
			Expect(app).Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))
			Expect(app).Should(appFixture.HaveHealthStatusCode(health.HealthStatusHealthy))

			By("verifying prod overlay patch was applied to the synced ConfigMap")
			syncedCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "source-hydrator-test",
					Namespace: ns.Name,
				},
			}
			Expect(syncedCM).Should(configmapFixture.HaveStringDataKeyValue("foo", "prod"))

			By("verifying hydrated branch contains rendered manifests")
			Eventually(func(g Gomega) {
				g.Expect(repo.CheckoutBranch("hydrated")).To(Succeed())
				manifest, err := repo.ReadFile("app/manifest.yaml")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(manifest).To(ContainSubstring("kind: ConfigMap"))
				g.Expect(manifest).To(ContainSubstring("name: source-hydrator-test"))
				g.Expect(manifest).To(ContainSubstring("foo: prod"))
				g.Expect(manifest).NotTo(ContainSubstring("foo: base"))
			}).Should(Succeed())
		})
	})
})
