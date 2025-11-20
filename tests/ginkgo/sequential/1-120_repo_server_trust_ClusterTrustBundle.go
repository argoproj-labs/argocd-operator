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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"regexp"
	"strconv"
	"strings"
	"time"

	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/pod"
	"gopkg.in/yaml.v2"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1alpha1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-120_repo_server_trust_ClusterTrustBundle", func() {

		var (
			k8sClient client.Client
			ctx       context.Context

			trustedHelmAppSource = &appv1alpha1.ApplicationSource{
				RepoURL:        "https://stefanprodan.github.io/podinfo",
				Chart:          "podinfo",
				TargetRevision: "6.5.3",
				Helm:           &appv1alpha1.ApplicationSourceHelm{Values: ""},
			}

			untrustedHelmAppSource = &appv1alpha1.ApplicationSource{
				RepoURL:        "https://helm.nginx.com/stable",
				Chart:          "nginx",
				TargetRevision: "1.1.0",
				Helm:           &appv1alpha1.ApplicationSourceHelm{Values: "service:\n          type: ClusterIP"},
			}
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that incorrect file suffix cannot be used for ClusterTrustBundle", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with wrong CTB path suffix")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
							ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{
								{Name: ptr.To("wrong-suffix"), Path: "wrong-suffix.pem"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			// The operator log contains the reason why reconciliation failed
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "invalid ClusterTrustBundle path suffix 'wrong-suffix.pem' in argocd, must be .crt",
				Reason:  "ErrorOccurred",
				Status:  "False",
				Type:    "Reconciled",
			}))
			Consistently(argoCD, "30s", "5s").Should(argocdFixture.HaveRepoStatus("Unknown"))
			Consistently(argoCD, "30s", "5s").ShouldNot(argocdFixture.BeAvailable())
		})

		It("ensures that missing ClusterTrustBundle aborts startup", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with missing CTB")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
							ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{
								{Name: ptr.To("no-such-ctb"), Path: "good-name.crt"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveServerStatus("Running"))
			Consistently(argoCD, "30s", "5s").Should(argocdFixture.HaveRepoStatus("Pending"))
			Consistently(argoCD, "30s", "5s").ShouldNot(argocdFixture.BeAvailable())
		})

		It("ensures that ClusterTrustBundles are trusted in repo-server and plugins", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			// Create a bundle with 2 CA certs in it. Ubuntu's update-ca-certificates issues a warning, but apparently it works
			// It is desirable to test with multiple certs in one bundle because OpenShift permits it
			combinedCtb := createCtbFromHost("github.com", "github.io")
			k8sClient.Delete(ctx, combinedCtb)
			defer k8sClient.Delete(ctx, combinedCtb)
			Expect(k8sClient.Create(ctx, combinedCtb)).To(Succeed())

			pluginCm, pluginContainer, pluginVolumes := createGitPullingPlugin(ns)
			Expect(k8sClient.Create(ctx, pluginCm)).To(Succeed())

			By("creating Argo CD instance trusting CTBs")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
							DropImageAnchors: true, // So we can test against upstream sites that would otherwise be trusted by the image
							ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{
								{Name: ptr.To(combinedCtb.Name), Path: "combined.crt"},
								{Name: ptr.To("no-such-ctb"), Path: "no-such-ctb.crt", Optional: ptr.To(true)},
							},
						},
						// plugin containers/volumes - this is not related to CTBs
						Volumes: pluginVolumes,
						SidecarContainers: []corev1.Container{
							*pluginContainer,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			initContainerLog := getRepoCertGenerationLog(k8sClient, ns)
			Expect(initContainerLog).Should(ContainSubstring("combined.crt"))
			Expect(initContainerLog).Should(ContainSubstring("no-such-ctb.crt"))
			Expect(getPodCertFileCount(k8sClient, ns)).Should(Equal(3))

			untrustedHelmApp := createHelmApp(ns, untrustedHelmAppSource)
			Expect(k8sClient.Create(ctx, untrustedHelmApp)).To(Succeed())

			// Using some host not trusted by github's intermediate cert. Gitlab-somewhat surprisingly-is.
			untrustedPluginApp := createPluginApp(ns, pluginCm, "https://kernel.googlesource.com/pub/scm/docs/man-pages/website.git")
			Expect(k8sClient.Create(ctx, untrustedPluginApp)).To(Succeed())

			trustedHelmApp := createHelmApp(ns, trustedHelmAppSource)
			Expect(k8sClient.Create(ctx, trustedHelmApp)).To(Succeed())

			trustedPluginApp := createPluginApp(ns, pluginCm, "https://github.com/argoproj-labs/argocd-operator.git")
			Expect(k8sClient.Create(ctx, trustedPluginApp)).To(Succeed())

			// Sleep to make sure the apps sync took place - otherwise there might be no conditions _yet_
			time.Sleep(20 * time.Second)

			Expect(untrustedHelmApp).Should(
				appFixture.HaveConditionMatching("ComparisonError", ".*failed to fetch chart.*"),
			)
			Expect(untrustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))

			Expect(untrustedPluginApp).Should(
				appFixture.HaveConditionMatching("ComparisonError", ".*certificate signed by unknown authority.*"),
			)
			Expect(untrustedPluginApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))

			Expect(trustedHelmApp).Should(appFixture.HaveNoConditions())
			Expect(trustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))

			Expect(trustedPluginApp).Should(appFixture.HaveNoConditions())
			Expect(trustedPluginApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))
		})

		It("ensures that empty ClusterTrustBundles with DropImageAnchors trusts nothing", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with empty system trust")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
							DropImageAnchors:    true,
							ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			Expect(getPodCertFileCount(k8sClient, ns)).Should(Equal(0))

			trustedHelmApp := createHelmApp(ns, trustedHelmAppSource)
			Expect(k8sClient.Create(ctx, trustedHelmApp)).To(Succeed())

			// Sleep to make sure the apps sync took place - otherwise there might be no conditions _yet_
			time.Sleep(20 * time.Second)

			Expect(trustedHelmApp).Should(appFixture.HaveConditionMatching(
				"ComparisonError",
				".*tls: failed to verify certificate: x509: certificate signed by unknown authority.*",
			))
			Expect(trustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))
		})

		It("ensures that empty ClusterTrustBundles with DropImageAnchors trusts nothing", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with empty system trust")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Repo: argov1beta1api.ArgoCDRepoSpec{
						SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
							DropImageAnchors:    false, // Keep the image ones
							ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			Expect(getPodCertFileCount(k8sClient, ns)).Should(BeNumerically(">", 100))
		})
	})
})

func printObject(k8sClient client.Client, ctx context.Context, obj client.Object) {
	err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	obj.SetManagedFields([]metav1.ManagedFieldsEntry{}) // Ignore
	Expect(err).To(BeNil())
	bytes, err := yaml.Marshal(obj)
	Expect(err).To(BeNil())
	println(string(bytes))
}

func createGitPullingPlugin(ns *corev1.Namespace) (*corev1.ConfigMap, *corev1.Container, []corev1.Volume) {
	By("Creating ConfigManagementPlugin resources for git clone")
	name := "cmp-git-https"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns.Name,
			Labels: map[string]string{
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"plugin.yaml": `apiVersion: argoproj.io/v1alpha1
kind: ConfigManagementPlugin
metadata:
  name: git-https
spec:
  version: v1.0
  generate:
    command: [bash, -c]
    args:
      - |
        set -euxo pipefail
        git clone --depth 1 --verbose "$ARGOCD_APP_SOURCE_REPO_URL"
`,
		},
	}

	container := &corev1.Container{
		Name:    name,
		Command: []string{"/var/run/argocd/argocd-cmp-server"},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/var/run/argocd",
				Name:      "var-files",
			},
			{
				MountPath: "/home/argocd/cmp-server/plugins",
				Name:      "plugins",
			},
			{
				MountPath: "/home/argocd/cmp-server/config/plugin.yaml",
				SubPath:   "plugin.yaml",
				Name:      name + "-config",
			},
			{
				MountPath: "/tmp",
				Name:      name + "-tmp",
			},
		},
	}

	volumes := []corev1.Volume{
		{
			Name: name + "-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: name,
					},
				},
			},
		},
		{
			Name: name + "-tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	return cm, container, volumes
}

func createHelmApp(ns *corev1.Namespace, source *appv1alpha1.ApplicationSource) *appv1alpha1.Application {
	By("creating helm Application " + source.Chart)

	return &appv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      source.Chart,
			Namespace: ns.Name,
		},
		Spec: appv1alpha1.ApplicationSpec{
			Project: "default",
			Source:  source,
			Destination: appv1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: ns.Name,
			},
			SyncPolicy: &appv1alpha1.SyncPolicy{
				Automated: &appv1alpha1.SyncPolicyAutomated{
					Prune: true, SelfHeal: true,
				},
			},
		},
	}
}

func createPluginApp(ns *corev1.Namespace, plugin *corev1.ConfigMap, url string) *appv1alpha1.Application {
	name := regexp.MustCompile("[^a-z]+").ReplaceAllString(url, "-")
	By("creating plugin Application " + name)
	return &appv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns.Name,
		},
		Spec: appv1alpha1.ApplicationSpec{
			Project: "default",
			Destination: appv1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: ns.Name,
			},
			Source: &appv1alpha1.ApplicationSource{
				RepoURL:        url,
				TargetRevision: "HEAD",
				Path:           ".",
				Plugin: &appv1alpha1.ApplicationSourcePlugin{
					Name: "git-https-v1.0",
					Env: appv1alpha1.Env{
						&appv1alpha1.EnvEntry{
							Name:  "ARGOCD_APP_SOURCE_REPO_URL",
							Value: url,
						},
					},
				},
			},
		},
	}
}

func createCtbFromHost(hosts ...string) *certificatesv1alpha1.ClusterTrustBundle {
	By("creating ClusterTrustBundle for " + strings.Join(hosts, ", "))

	bundle := []string{}
	for _, host := range hosts {
		bundle = append(bundle, getCACert(host))
	}

	return &certificatesv1alpha1.ClusterTrustBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterTrustBundle",
			APIVersion: "certificates.k8s.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "global-ca-trust-bundle",
		},
		Spec: certificatesv1alpha1.ClusterTrustBundleSpec{
			TrustBundle: strings.Join(bundle, "\n"),
		},
	}
}

func getCACert(host string) string {
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{})
	Expect(err).To(BeNil())
	defer conn.Close()

	pcs := conn.ConnectionState().PeerCertificates

	// ClusterTrustBundle cannot hold leaf certificates, so testing with CA cert at least.
	return encodeCert(pcs[len(pcs)-1])
}

func encodeCert(cert *x509.Certificate) string {
	writer := strings.Builder{}
	err := pem.Encode(&writer, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	Expect(err).To(BeNil())

	return writer.String()
}

func getPodCertFileCount(k8sClient client.Client, ns *corev1.Namespace) int {
	rsPod := pod.GetPodByNameRegexp(k8sClient, regexp.MustCompile(".*-repo-server.*"), client.InNamespace(ns.Name))
	out, err := osFixture.ExecCommandWithOutputParam(
		false,
		"kubectl", "-n", ns.Name, "exec", "-c", "argocd-repo-server", rsPod.Name, "--",
		// Using `ls -1` (one) for counting because `ls -l` produces a "total" line
		"bash", "-c", "ls -1 /etc/ssl/certs | wc -l",
	)

	fileCount, err := strconv.Atoi(strings.TrimSpace(out))
	Expect(err).To(BeNil())

	return fileCount
}

func getRepoCertGenerationLog(k8sClient client.Client, ns *corev1.Namespace) string {
	rsPod := pod.GetPodByNameRegexp(k8sClient, regexp.MustCompile(".*-repo-server.*"), client.InNamespace(ns.Name))
	out, err := osFixture.ExecCommandWithOutputParam(
		false,
		"kubectl", "-n", ns.Name, "logs", "-c", "update-ca-certificates", rsPod.Name,
	)
	Expect(err).To(BeNil())
	return out
}
