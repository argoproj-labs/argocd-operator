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
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
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

		It("Ensure commit server is activated by Source Hydrator config", func() {
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
					Eventually(csSA, "30s", "5s").Should(exist)
				} else {
					haveStatus := argocdFixture.HaveCommitServerStatus("Unknown")
					Eventually(argoCD, "30s", "5s").Should(haveStatus)
					Consistently(argoCD, "10s", "2s").Should(haveStatus)
					Expect(csService).ShouldNot(exist)
					Expect(csNetPol).ShouldNot(exist)
					Eventually(csSA, "30s", "5s").ShouldNot(exist)
				}

			}

			By("Not running by default")
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			assertRunning(false)

			By("Running when enabled")
			argoCD.Spec.SourceHydrator.Enabled = ptr.To(true)
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			assertRunning(true)

			By("Not running when disabled")
			argoCD.Spec.SourceHydrator.Enabled = ptr.To(false)
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			assertRunning(false)
		})

		It("should apply CommitServer configuration options to the running CommitServer deployment", func() {
			ns, cleanup = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			expectedCommand := []string{"/usr/local/bin/argocd-commit-server", "--loglevel", "info", "--logformat", "json"}
			deploymentObjectKey := client.ObjectKey{Namespace: ns.Name, Name: "example-cs-options-commit-server"}
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
					Name:      "example-cs-options",
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
			argoCD.Spec.CommitServer.Annotations = nil
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() map[string]string {
				return fetchDeployment().Spec.Template.Annotations
			}, "5s", "1s").ToNot(HaveKey("example-annotation"))

			By("Labels reconciles")
			argoCD.Spec.CommitServer.Labels = nil
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() map[string]string {
				return fetchDeployment().Spec.Template.Labels
			}, "5s", "1s").ToNot(HaveKey("example-label"))

			By("Env reconciles")
			argoCD.Spec.CommitServer.Env = nil
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() []corev1.EnvVar {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Env
			}, "5s", "1s").ToNot(ContainElement(corev1.EnvVar{Name: "FOO", Value: "BAR"}))

			By("Resources reconciles")
			argoCD.Spec.CommitServer.Resources.Limits = nil
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() corev1.ResourceRequirements {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Resources
			}, "5s", "1s").To(Equal(corev1.ResourceRequirements{Requests: resources.Requests})) // Same Requests, no Limits

			By("InitContainers reconciles")
			argoCD.Spec.CommitServer.InitContainers[0].Command = []string{"echo", "init-2"}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() []string {
				return fetchDeployment().Spec.Template.Spec.InitContainers[0].Command
			}, "5s", "1s").To(Equal([]string{"echo", "init-2"}))

			By("Command reconciles")
			argoCD.Spec.CommitServer.LogLevel = "debug"
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(func() []string {
				return fetchDeployment().Spec.Template.Spec.Containers[0].Command
			}, "5s", "1s").To(ContainElement("debug"))
		})
	})
})
