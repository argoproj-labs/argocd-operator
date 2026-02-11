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
	"fmt"
	"regexp"
	"strings"

	"github.com/onsi/gomega/gcustom"
	matcher "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"

	"k8s.io/utils/ptr"

	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var (
	// The differences between the upstream image using Ubuntu, and the downstream one using rhel.
	image        = "" // argocd-operator default
	version      = "" // argocd-operator default
	caBundlePath = "/etc/ssl/certs/ca-certificates.crt"

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

	k8sClient client.Client
	ctx       context.Context

	clusterSupportsClusterTrustBundles bool
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-120_repo_server_system_ca_trust", func() {
		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

			clusterSupportsClusterTrustBundles = detectClusterTrustBundleSupport(k8sClient, ctx)
		})

		AfterEach(func() {
			purgeCtbs()
		})

		It("ensures that missing Secret aborts startup", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with missing Secret")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					Secrets: []corev1.SecretProjection{
						{LocalObjectReference: corev1.LocalObjectReference{Name: "no-such-secret"}},
					},
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "1m", "5s").Should(argocdFixture.HaveServerStatus("Running"))
			Consistently(argoCD, "20s", "5s").Should(argocdFixture.HaveRepoStatus("Pending"))
			Expect(argoCD).ShouldNot(argocdFixture.BeAvailable())
		})

		It("ensures that ClusterTrustBundles are trusted in repo-server and plugins", func() {
			if !clusterSupportsClusterTrustBundles {
				Skip("Cluster does not support ClusterTrustBundles")
			}

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			// Create a bundle with 2 CA certs in it. Ubuntu's update-ca-certificates issues a warning, but apparently it works
			// It is desirable to test with multiple certs in one bundle because OpenShift permits it
			combinedCtb := createCtbFromCerts(getCACert("github.com"), getCACert("github.io"))
			_ = k8sClient.Delete(ctx, combinedCtb) // Exists only in case of previous failures
			defer func() { _ = k8sClient.Delete(ctx, combinedCtb) }()
			Expect(k8sClient.Create(ctx, combinedCtb)).To(Succeed())

			pluginCm, pluginContainer, pluginVolumes := createGitPullingPlugin(ns)
			Expect(k8sClient.Create(ctx, pluginCm)).To(Succeed())

			By("creating Argo CD instance trusting CTBs")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true, // So we can test against upstream sites that would otherwise be trusted by the image
					ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{
						{Name: ptr.To(combinedCtb.Name), Path: "combined.crt"},
						{Name: ptr.To("nah"), Path: "no-such-ctb.crt", Optional: ptr.To(true)},
					},
				},
				// plugin containers/volumes - this is not related to CTBs
				Volumes: pluginVolumes,
				SidecarContainers: []corev1.Container{
					*pluginContainer,
				},
			})

			By("verifying correctly established system trust")
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			verifyCorrectlyConfiguredTrust(ns)
			Expect(repoServerSystemCaTrust(ns)).Should(trustCerts(Equal(2), And(
				ContainSubstring("combined.crt"),
				ContainSubstring("no-such-ctb.crt"),
			)))
		})

		It("ensures that CMs and Secrets are trusted in repo-server and plugins", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			cmCert := createCmFromCert(ns, getCACert("github.com"))
			Expect(k8sClient.Create(ctx, cmCert)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, cmCert) }()
			secretCert := createSecretFromCert(ns, getCACert("github.io"))
			Expect(k8sClient.Create(ctx, secretCert)).To(Succeed())
			defer func() { _ = k8sClient.Delete(ctx, secretCert) }()

			pluginCm, pluginContainer, pluginVolumes := createGitPullingPlugin(ns)
			Expect(k8sClient.Create(ctx, pluginCm)).To(Succeed())

			By("creating Argo CD instance trusting CTBs")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true, // So we can test against upstream sites that would otherwise be trusted by the image
					Secrets: []corev1.SecretProjection{{
						// No Items, Map all
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretCert.Name,
						},
					}},
					ConfigMaps: []corev1.ConfigMapProjection{{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cmCert.Name,
						},
						Optional: ptr.To(true),
						Items: []corev1.KeyToPath{
							{Key: "ca.cm.crt", Path: "ca.cm.wrong-suffix"},
						},
					}},
				},
				// plugin containers/volumes - this is not related to Secret/CM
				Volumes: pluginVolumes,
				SidecarContainers: []corev1.Container{
					*pluginContainer,
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			initContainerLog := getRepoCertGenerationLog(findRunningRepoServerPod(k8sClient, ns))
			Expect(initContainerLog).Should(ContainSubstring("ca.secret.crt"))
			Expect(initContainerLog).Should(ContainSubstring("ca.cm.wrong-suffix.crt"))
			verifyCorrectlyConfiguredTrust(ns)
		})

		It("ensures that 0 trusted certs with DropImageCertificates trusts nothing", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with empty system trust")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				// Remount /tmp to make sure the init container can handle that
				Volumes: []corev1.Volume{{
					Name:         "user-provided-tmp",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				}},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "user-provided-tmp", ReadOnly: false, MountPath: "/tmp"},
				},
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true,
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			Expect(repoServerSystemCaTrust(ns)).Should(trustCerts(Equal(0), Not(BeEmpty())))

			trustedHelmApp := createHelmApp(ns, trustedHelmAppSource)
			Expect(k8sClient.Create(ctx, trustedHelmApp)).To(Succeed())

			Eventually(trustedHelmApp, "20s", "5s").Should(appFixture.HaveConditionMatching(
				"ComparisonError",
				".*tls: failed to verify certificate: x509: certificate signed by unknown authority.*",
			))
			Expect(trustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))
		})

		It("ensures that empty trust keeps image certs in place", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with empty system trust")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: false, // Keep the image ones
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Expect(repoServerSystemCaTrust(ns)).Should(trustCerts(BeNumerically(">", 100), Not(BeEmpty())))
		})

		It("ensures that Secrets and ConfigMaps get reconciled", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating Argo CD instance with empty system trust, but full of anticipation")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true, // To make the counting easier
					Secrets: []corev1.SecretProjection{{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "ca-trust",
						},
						Optional: ptr.To(true),
					}},
					ConfigMaps: []corev1.ConfigMapProjection{{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "ca-trust",
						},
						Optional: ptr.To(true),
					}},
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			actualTrust := repoServerSystemCaTrust(ns)
			Expect(actualTrust).Should(trustCerts(Equal(0), Not(BeEmpty())))

			By("creating ConfigMap with 1 cert")
			cmCert := createCmFromCert(ns, getCACert("github.com"))
			defer func() { _ = k8sClient.Delete(ctx, cmCert) }()
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Create(ctx, cmCert)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(1), Not(BeEmpty())))

			By("creating Secret with 1 cert")
			secretCert := createSecretFromCert(ns, getCACert("github.io"))
			defer func() { _ = k8sClient.Delete(ctx, secretCert) }()
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Create(ctx, secretCert)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(2), Not(BeEmpty())))

			By("updating ConfigMap to 2 certs")
			expectReconcile(k8sClient, ns, true, func() {
				configmapFixture.Update(cmCert, func(configMap *corev1.ConfigMap) {
					configMap.Data = map[string]string{
						"a.crt": getCACert("github.com"),
						"b.crt": getCACert("google.com"),
					}
				})
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(3), Not(BeEmpty())))

			By("updating Secret to 0 certs")
			expectReconcile(k8sClient, ns, true, func() {
				secretFixture.Update(secretCert, func(secret *corev1.Secret) {
					// Albeit `.Data` is never written by the test, it is the field that holds the data after the Create/Get roundtrip.
					// Erase, otherwise reducing the content of `.StringData` does not have the expected effect.
					secret.Data = map[string][]byte{}
					secret.StringData = map[string]string{}
				})
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(2), Not(BeEmpty())))

			By("updating ConfigMap to 1 certs")
			expectReconcile(k8sClient, ns, true, func() {
				configmapFixture.Update(cmCert, func(configMap *corev1.ConfigMap) {
					configMap.Data = map[string]string{
						"a.crt": getCACert("redhat.com"),
					}
				})
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(1), Not(BeEmpty())))

			By("deleting ConfigMap")
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Delete(ctx, cmCert)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(0), Not(BeEmpty())))
		})

		It("ensures that ClusterTrustBundles get reconciled", func() {
			if !clusterSupportsClusterTrustBundles {
				Skip("Cluster does not support ClusterTrustBundles")
			}

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			combinedCtb := createCtbFromCerts(getCACert("github.com"), getCACert("github.io"))
			_ = k8sClient.Delete(ctx, combinedCtb) // Exists only in case of previous failures, must be deleted before argo starts!

			By("creating Argo CD instance with empty system trust, but full of anticipation")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true, // To make the counting easier
					ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{{
						Name: ptr.To(combinedCtb.Name), Path: "ctb.crt", Optional: ptr.To(true),
					}},
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			actualTrust := repoServerSystemCaTrust(ns)
			Expect(actualTrust).Should(trustCerts(Equal(0), Not(BeEmpty())), actualTrust.diagnose())

			By("creating ClusterTrustBundle with 2 certs")
			defer func() { _ = k8sClient.Delete(ctx, combinedCtb) }()
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Create(ctx, combinedCtb)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(2), Not(BeEmpty())), actualTrust.diagnose())

			By("updating ClusterTrustBundle with 1 cert")
			expectReconcile(k8sClient, ns, true, func() {
				ctbUpdate(combinedCtb, func(bundle *certificatesv1beta1.ClusterTrustBundle) {
					bundle.Spec = certificatesv1beta1.ClusterTrustBundleSpec{
						SignerName:  bundle.Spec.SignerName,
						TrustBundle: getCACert("github.com"),
					}
				})
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "6m", "15s").Should(trustCerts(Equal(1), Not(BeEmpty())), actualTrust.diagnose())

			By("deleting ClusterTrustBundle")
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Delete(ctx, combinedCtb)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "6m", "15s").Should(trustCerts(Equal(0), Not(BeEmpty())), actualTrust.diagnose())
		})

		It("only relevant ClusterTrustBundles changes get reconciled", func() {
			if !clusterSupportsClusterTrustBundles {
				Skip("Cluster does not support ClusterTrustBundles")
			}

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			// Use random label value not to collide with leftover CTBs fom other tests
			labelVal := rand.String(5)
			signerName := "acme.com/signer"
			By("creating Argo CD instance with system trust")
			argoCD := argoCDSpec(ns, argov1beta1api.ArgoCDRepoSpec{
				SystemCATrust: &argov1beta1api.ArgoCDSystemCATrustSpec{
					DropImageCertificates: true, // To make the counting easier
					// Test CTB update detection based on CTB binding specified by labels - no real signers involved
					ClusterTrustBundles: []corev1.ClusterTrustBundleProjection{
						{
							SignerName: ptr.To(signerName),
							LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
								"test": labelVal,
							}},
							Path:     "one.crt",
							Optional: ptr.To(true),
						},
					},
				},
			})
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("adding ClusterTrustBundle with 1 cert")
			oneCtb := createCtbFromCerts(getCACert("github.com"))
			oneCtb.Labels["test"] = labelVal
			oneCtb.Name = "acme.com:signer:repo-server-system-ca-trust-test-one"
			oneCtb.Spec.SignerName = signerName
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Create(ctx, oneCtb)).To(Succeed())
			})
			actualTrust := repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(1), Not(BeEmpty())), actualTrust.diagnose())

			By("adding ClusterTrustBundle with other cert")
			twoCtb := createCtbFromCerts(getCACert("github.io"))
			twoCtb.Labels["test"] = labelVal
			twoCtb.Name = "acme.com:signer:repo-server-system-ca-trust-test-two"
			twoCtb.Spec.SignerName = signerName
			expectReconcile(k8sClient, ns, true, func() {
				Expect(k8sClient.Create(ctx, twoCtb)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Eventually(actualTrust, "30s", "5s").Should(trustCerts(Equal(2), Not(BeEmpty())), actualTrust.diagnose())

			By("updating Argo CD to read from ClusterTrustBundle that does not exist")
			expectReconcile(k8sClient, ns, true, func() {
				argocdFixture.Update(argoCD, func(cd *argov1beta1api.ArgoCD) {
					cd.Spec.Repo.SystemCATrust.ClusterTrustBundles = []corev1.ClusterTrustBundleProjection{
						{
							Name:     ptr.To("no-such-ctb"),
							Path:     "three.crt",
							Optional: ptr.To(true),
						},
					}
				})
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Consistently(actualTrust, "10s", "5s").Should(trustCerts(Equal(0), Not(BeEmpty())), actualTrust.diagnose())

			By("creating unrelated ClusterTrustBundle")
			fourCtb := createCtbFromCerts(getCACert("google.com"))
			expectReconcile(k8sClient, ns, false, func() {
				Expect(k8sClient.Create(ctx, fourCtb)).To(Succeed())
			})
			actualTrust = repoServerSystemCaTrust(ns)
			Consistently(actualTrust, "10s", "5s").Should(trustCerts(Equal(0), Not(BeEmpty())), actualTrust.diagnose())
		})
	})
})

func ctbUpdate(obj *certificatesv1beta1.ClusterTrustBundle, modify func(*certificatesv1beta1.ClusterTrustBundle)) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}

		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(context.Background(), obj)
	})
	Expect(err).ToNot(HaveOccurred())
}

func argoCDSpec(ns *corev1.Namespace, repoSpec argov1beta1api.ArgoCDRepoSpec) *argov1beta1api.ArgoCD {
	return &argov1beta1api.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
		Spec: argov1beta1api.ArgoCDSpec{
			Image:   image,
			Version: version,
			Repo:    repoSpec,
		},
	}
}

func detectClusterTrustBundleSupport(k8sClient client.Client, ctx context.Context) bool {
	err := k8sClient.List(ctx, &certificatesv1beta1.ClusterTrustBundleList{})
	if _, ok := err.(*apiutil.ErrResourceDiscoveryFailed); ok {
		return false
	}
	Expect(err).ToNot(HaveOccurred()) // Every other error is an error
	return true
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

func createPluginApp(ns *corev1.Namespace, url string) *appv1alpha1.Application {
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

func createCtbFromCerts(bundle ...string) *certificatesv1beta1.ClusterTrustBundle {
	return &certificatesv1beta1.ClusterTrustBundle{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterTrustBundle",
			APIVersion: "certificates.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "repo-server-system-ca-trust",
			Labels: map[string]string{
				"argocd-operator-test": "repo_server_system_ca_trust",
			},
		},
		Spec: certificatesv1beta1.ClusterTrustBundleSpec{
			TrustBundle: strings.Join(bundle, "\n"),
		},
	}
}

func createCmFromCert(ns *corev1.Namespace, bundle string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ca-trust",
			Namespace: ns.Name,
		},
		Data: map[string]string{
			"ca.cm.crt": bundle,
		},
	}
}

func createSecretFromCert(ns *corev1.Namespace, bundle string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ca-trust",
			Namespace: ns.Name,
		},
		Type: "Opaque",
		StringData: map[string]string{
			"ca.secret.crt": bundle,
		},
	}
}

func getCACert(host string) string {
	config := &tls.Config{MinVersion: tls.VersionTLS13}
	conn, err := tls.Dial("tcp", host+":443", config)
	Expect(err).ToNot(HaveOccurred())
	defer func() { _ = conn.Close() }()

	pcs := conn.ConnectionState().PeerCertificates

	// ClusterTrustBundle cannot hold leaf certificates, so testing with CA cert at least. In theory, some of the hosts
	// we test against can share the same CA cert, so albeit not likely, rudimentary negative testing is needed.
	return encodeCert(pcs[len(pcs)-1])
}

func encodeCert(cert *x509.Certificate) string {
	writer := strings.Builder{}
	err := pem.Encode(&writer, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	Expect(err).ToNot(HaveOccurred())

	return writer.String()
}

type podTrust struct {
	ns        *corev1.Namespace
	k8sClient client.Client

	count  int
	log    string
	events string
}

func (pt *podTrust) fetch() {
	pod := findRunningRepoServerPod(pt.k8sClient, pt.ns)
	pt.count = getTrustedCertCount(pod)
	pt.log = getRepoCertGenerationLog(pod)

	out, err := osFixture.ExecCommandWithOutputParam(false, false, "kubectl", "-n", pt.ns.Name, "events")
	if err != nil {
		Expect(err).NotTo(HaveOccurred())
	}
	pt.events = out
}

func (pt *podTrust) diagnose() string {
	return fmt.Sprintf(
		"System CA Trust init container log:\n%s\nProject events:\n%s\n",
		pt.log, pt.events,
	)
}

func repoServerSystemCaTrust(ns *corev1.Namespace) *podTrust {
	return &podTrust{ns: ns, k8sClient: k8sClient}
}

// expectReconcile makes sure the action has either caused or not caused the repo server to reconcile
func expectReconcile(k8sClient client.Client, ns *corev1.Namespace, reconcile bool, action func()) {
	podNameFunc := func() string {
		return findRunningRepoServerPod(k8sClient, ns).Name
	}
	oldPodName := podNameFunc()

	By(fmt.Sprintf("Expecting reconcile of old pod %s: %v", oldPodName, reconcile))

	action()

	if reconcile {
		Eventually(podNameFunc, "30s", "5s").
			WithOffset(1).
			Should(Not(Equal(oldPodName)), "expected pod to reconcile")
	} else {
		Consistently(podNameFunc, "30s", "5s").
			WithOffset(1).
			Should(Equal(oldPodName), "expected pod not to reconcile")
	}
}

func trustCerts(countMatcher, logMatcher matcher.GomegaMatcher) matcher.GomegaMatcher {
	// Wrap to capture and attach diagnostics
	matchCount := gcustom.MakeMatcher(func(pt *podTrust) (bool, error) {
		// call fetch exactly once so `count` and `log` is populated
		pt.fetch()

		success, err := countMatcher.Match(pt.count)
		if err != nil {
			return false, err
		}
		if success {
			return true, nil
		}

		return false, fmt.Errorf(
			"%s\n\n--- Diagnostics ---\n%s\n===",
			countMatcher.FailureMessage(pt.count),
			pt.diagnose(),
		)
	})

	matchLog := WithTransform(func(pt *podTrust) string {
		return pt.log
	}, logMatcher)

	return And(matchCount, matchLog)
}

func getTrustedCertCount(rsPod *corev1.Pod) int {
	command := []string{
		"kubectl", "-n", rsPod.Namespace, "exec",
		"-c", "argocd-repo-server", rsPod.Name, "--",
		"cat", caBundlePath,
	}

	var out string
	var err error
	Eventually(func() error {
		out, err = osFixture.ExecCommandWithOutputParam(false, false, command...)
		return err
	}, "5s", "1s").Should(Succeed())
	Expect(err).ToNot(HaveOccurred(), out)

	seen := make(map[string]bool)
	var currentBlock strings.Builder
	for line := range strings.Lines(out) {
		switch {
		case strings.Contains(line, "BEGIN CERTIFICATE"):
			currentBlock.Reset()
		case strings.Contains(line, "END CERTIFICATE"):
			seen[currentBlock.String()] = true
		default:
			currentBlock.WriteString(line)
		}
	}
	return len(seen)
}

func getRepoCertGenerationLog(rsPod *corev1.Pod) string {
	out, err := osFixture.ExecCommandWithOutputParam(
		false,
		false,
		"kubectl", "-n", rsPod.Namespace, "logs", "-c", "update-ca-certificates", rsPod.Name,
	)
	Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("output: %s", out))
	return out
}

func findRunningRepoServerPod(k8sClient client.Client, ns *corev1.Namespace) *corev1.Pod {
	nameRegexp := regexp.MustCompile(".*-repo-server.*")

	var pod *corev1.Pod
	Eventually(func() error {
		list := &corev1.PodList{}
		if err := k8sClient.List(context.Background(), list, client.InNamespace(ns.Name)); err != nil {
			return err
		}

		var runningPods []*corev1.Pod
		for _, p := range list.Items {
			if p.Status.Phase == "Running" && nameRegexp.MatchString(p.Name) {
				pod := p // Create a new variable to avoid issues with loop variable capture
				runningPods = append(runningPods, &pod)
			}
		}

		if len(runningPods) == 1 {
			pod = runningPods[0]
			return nil
		}
		return fmt.Errorf("expected exactly one running repo-server pod, found %d", len(runningPods))
	}, "20s", "2s").WithOffset(1).Should(Succeed(), "Failed to find Running repo-server pod")

	return pod
}

func verifyCorrectlyConfiguredTrust(ns *corev1.Namespace) {
	untrustedHelmApp := createHelmApp(ns, untrustedHelmAppSource)
	Expect(k8sClient.Create(ctx, untrustedHelmApp)).To(Succeed())

	// Using some host not trusted by github's intermediate cert. Gitlab-somewhat surprisingly-is.
	untrustedPluginApp := createPluginApp(ns, "https://kernel.googlesource.com/pub/scm/docs/man-pages/website.git")
	Expect(k8sClient.Create(ctx, untrustedPluginApp)).To(Succeed())

	trustedHelmApp := createHelmApp(ns, trustedHelmAppSource)
	Expect(k8sClient.Create(ctx, trustedHelmApp)).To(Succeed())

	trustedPluginApp := createPluginApp(ns, "https://github.com/argoproj-labs/argocd-operator.git")
	Expect(k8sClient.Create(ctx, trustedPluginApp)).To(Succeed())

	Eventually(func(g Gomega) {
		g.Expect(untrustedHelmApp).Should(
			appFixture.HaveConditionMatching("ComparisonError", ".*failed to fetch chart.*"),
		)
		g.Expect(untrustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))

		g.Expect(untrustedPluginApp).Should(
			appFixture.HaveConditionMatching("ComparisonError", ".*certificate signed by unknown authority.*"),
		)
		g.Expect(untrustedPluginApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeUnknown))

		g.Expect(trustedHelmApp).Should(appFixture.HaveNoConditions())
		g.Expect(trustedHelmApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))

		g.Expect(trustedPluginApp).Should(appFixture.HaveNoConditions())
		g.Expect(trustedPluginApp).Should(appFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeSynced))
	}, "20s", "5s").Should(Succeed())
}

// purgeCtbs deletes all of the cluster-wide resource, that can get leaked on test failure/abort.
func purgeCtbs() {
	if clusterSupportsClusterTrustBundles {
		expr := client.MatchingLabels{"argocd-operator-test": "repo_server_system_ca_trust"}
		Expect(k8sClient.DeleteAllOf(ctx, &certificatesv1beta1.ClusterTrustBundle{}, expr)).To(Succeed())
	}
}
