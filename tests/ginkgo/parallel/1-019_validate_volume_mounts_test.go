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
	"k8s.io/utils/ptr"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-019_validate_volume_mounts", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates that Argo CD components have expected volumes and volume mounts", func() {

			By("creating new namespace-scoped Argo CD instance")
			randomNS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDRandomNS := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: randomNS.Name},
			}
			Expect(k8sClient.Create(ctx, argoCDRandomNS)).To(Succeed())

			Eventually(argoCDRandomNS, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying volumemounts and volumes of Argo CD Server")
			argocdServerDepl := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: randomNS.Name}}

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&argocdServerDepl), &argocdServerDepl)).To(Succeed())

			Expect(argocdServerDepl.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
				{Name: "tls-certs", MountPath: "/app/config/tls"},
				{Name: "argocd-repo-server-tls", MountPath: "/app/config/server/tls"},
				{Name: "argocd-operator-redis-tls", MountPath: "/app/config/server/tls/redis"},
				{Name: "plugins-home", MountPath: "/home/argocd"},
				{Name: "argocd-cmd-params-cm", MountPath: "/home/argocd/params"},
				{Name: "tmp", MountPath: "/tmp"},
			}))

			Expect(argocdServerDepl.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name: "ssh-known-hosts", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-ssh-known-hosts-cm"}},
					},
				},
				{
					Name: "tls-certs", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-tls-certs-cm"}},
					},
				},
				{
					Name: "argocd-repo-server-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-repo-server-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "argocd-operator-redis-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-operator-redis-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "plugins-home", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "argocd-cmd-params-cm", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							Items:                []corev1.KeyToPath{{Key: "server.profile.enabled", Path: "profiler.enabled"}},
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-cmd-params-cm"},
							Optional:             ptr.To(true)},
					},
				},
				{
					Name: "tmp", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}))

			By("verifying volumemounts and volumes of Argo CD Repo server")
			argocdRepoServerDepl := appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-repo-server", Namespace: randomNS.Name}}

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&argocdRepoServerDepl), &argocdRepoServerDepl)).To(Succeed())

			Expect(argocdRepoServerDepl.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
				{Name: "tls-certs", MountPath: "/app/config/tls"},
				{Name: "gpg-keys", MountPath: "/app/config/gpg/source"},
				{Name: "gpg-keyring", MountPath: "/app/config/gpg/keys"},
				{Name: "argocd-repo-server-tls", MountPath: "/app/config/reposerver/tls"},
				{Name: "argocd-operator-redis-tls", MountPath: "/app/config/reposerver/tls/redis"},
				{Name: "plugins", MountPath: "/home/argocd/cmp-server/plugins"},
				{Name: "tmp", MountPath: "/tmp"},
			}))

			Expect(argocdRepoServerDepl.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name: "ssh-known-hosts", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-ssh-known-hosts-cm"}},
					},
				},
				{
					Name: "tls-certs", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-tls-certs-cm"}},
					},
				},
				{
					Name: "gpg-keys", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-gpg-keys-cm"}},
					},
				},
				{
					Name: "gpg-keyring", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "argocd-repo-server-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-repo-server-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "argocd-operator-redis-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-operator-redis-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "var-files", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "plugins", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "tmp", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}))

			By("verifying volumemounts and volumes of Argo CD Application controller")
			applControllerSS := appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: argoCDRandomNS.Namespace}}

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&applControllerSS), &applControllerSS)).To(Succeed())

			Expect(applControllerSS.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{Name: "argocd-repo-server-tls", MountPath: "/app/config/controller/tls"},
				{Name: "argocd-operator-redis-tls", MountPath: "/app/config/controller/tls/redis"},
				{Name: "argocd-home", MountPath: "/home/argocd"},
				{Name: "argocd-cmd-params-cm", MountPath: "/home/argocd/params"},
				{Name: "argocd-application-controller-tmp", MountPath: "/tmp"},
			}))

			Expect(applControllerSS.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name: "argocd-repo-server-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-repo-server-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "argocd-operator-redis-tls", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "argocd-operator-redis-tls",
							Optional:    ptr.To(true),
						},
					},
				},
				{
					Name: "argocd-home", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "argocd-cmd-params-cm", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							Items: []corev1.KeyToPath{
								{Key: "controller.profile.enabled", Path: "profiler.enabled"},
								{Key: "controller.resource.health.persist", Path: "controller.resource.health.persist"},
							},
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-cmd-params-cm"},
							Optional:             ptr.To(true)},
					},
				},
				{
					Name: "argocd-application-controller-tmp", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}))

			By("adding volume to applicationset controller, and verifying volumemounts and volumes are set on Deployment")

			argocdFixture.Update(argoCDRandomNS, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet = &argov1beta1api.ArgoCDApplicationSet{
					Volumes: []corev1.Volume{
						{
							Name: "empty-dir-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "empty-dir-volume",
							MountPath: "/etc/test",
						},
					},
				}
			})

			Eventually(argoCDRandomNS, "2m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDRandomNS, "2m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			appSetDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-applicationset-controller",
					Namespace: argoCDRandomNS.Namespace,
				},
				Spec: appsv1.DeploymentSpec{},
			}
			Eventually(appSetDepl).Should(k8sFixture.ExistByName())

			Expect(appSetDepl.Spec.Template.Spec.Containers[0].VolumeMounts).Should(Equal([]corev1.VolumeMount{
				{
					Name:      "ssh-known-hosts",
					MountPath: "/app/config/ssh",
				},
				{
					Name:      "tls-certs",
					MountPath: "/app/config/tls",
				},
				{
					Name:      "gpg-keys",
					MountPath: "/app/config/gpg/source",
				},
				{
					Name:      "gpg-keyring",
					MountPath: "/app/config/gpg/keys",
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
				{
					Name:      "empty-dir-volume",
					MountPath: "/etc/test",
				},
			}))

			Expect(appSetDepl.Spec.Template.Spec.Volumes).Should(Equal([]corev1.Volume{
				{
					Name: "ssh-known-hosts",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "argocd-ssh-known-hosts-cm",
						},
						DefaultMode: ptr.To(int32(420)),
					}},
				},
				{
					Name: "tls-certs",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "argocd-tls-certs-cm",
						},
						DefaultMode: ptr.To(int32(420)),
					}},
				},
				{
					Name: "gpg-keys",
					VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "argocd-gpg-keys-cm",
						},
						DefaultMode: ptr.To(int32(420)),
					}},
				},
				{
					Name: "gpg-keyring",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "empty-dir-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}))

			By("adding emptydir volume to .spec.sso and verifying it is set on dex server Deployment")

			argocdFixture.Update(argoCDRandomNS, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
					Provider: argov1beta1api.SSOProviderTypeDex,
					Dex: &argov1beta1api.ArgoCDDexSpec{
						Config: "test-config",
						Volumes: []corev1.Volume{
							{Name: "empty-dir-volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "empty-dir-volume", MountPath: "/etc/test"},
						},
					},
				}
			})

			Eventually(argoCDRandomNS, "2m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDRandomNS, "2m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			dexDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-dex-server",
					Namespace: argoCDRandomNS.Namespace,
				},
			}
			Eventually(dexDeployment).Should(k8sFixture.ExistByName())

			Expect(dexDeployment.Spec.Template.Spec.InitContainers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "static-files",
					MountPath: "/shared",
				},
				{
					Name:      "dexconfig",
					MountPath: "/tmp",
				},
				{
					Name:      "empty-dir-volume",
					MountPath: "/etc/test",
				},
			}))

			Expect(dexDeployment.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "static-files",
					MountPath: "/shared",
				},
				{
					Name:      "dexconfig",
					MountPath: "/tmp",
				},
				{
					Name:      "empty-dir-volume",
					MountPath: "/etc/test",
				},
			}))

			Expect(dexDeployment.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
				{
					Name: "static-files",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "dexconfig",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},

				{
					Name: "empty-dir-volume",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}))

			Eventually(dexDeployment, "4m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

		})

	})
})
