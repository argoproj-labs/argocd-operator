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
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-104_validate_applicationset_tls_scm_volume_mount", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that specifying a ConfigMap to ArgoCD CR .spec.applicationSet.SCMRootCAConfigMap will cause that ConfigMap to be mounted into applicationset controller Deployment, and that applicationset controller has expected volumes and mounts", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating a certificate in a ConfigMap that we expect to be mounted into applicationset controller Deployment")
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "test-1-104-appsets-scm-tls-cm", Namespace: ns.Name},
				Data: map[string]string{
					"cert": `-----BEGIN CERTIFICATE-----
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
XWyb96wrUlv+E8I=
-----END CERTIFICATE-----`,
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			By("creating simple Argo CD instance that references the ConfigMap via .spec.applicationSet.scmRootCAConfigMap ")

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						SCMRootCAConfigMap: configMap.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("expecting application set controller to be running with parameter that references the cert via volume mount path")

			appsetDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-applicationset-controller", Namespace: ns.Name},
			}
			Expect(appsetDepl).To(deploymentFixture.HaveContainerCommandSubstring("--scm-root-ca-path /app/tls/scm/cert", 0))

			By("expecting that volume mount is mapped into container")
			container := deploymentFixture.GetTemplateSpecContainerByName("argocd-applicationset-controller", *appsetDepl)
			Expect(container).ToNot(BeNil())

			volumeMountMatch := false
			for _, vm := range container.VolumeMounts {
				if vm.Name == "appset-gitlab-scm-tls-cert" && vm.MountPath == "/app/tls/scm/" {
					volumeMountMatch = true
					break
				}
			}
			Expect(volumeMountMatch).To(BeTrue())

			By("expecting that ConfigMap is mounted into deployment as a volume")
			volumeMatch := false
			for _, v := range appsetDepl.Spec.Template.Spec.Volumes {
				if v.Name == "appset-gitlab-scm-tls-cert" {
					if v.ConfigMap != nil {

						if *v.ConfigMap.DefaultMode == 420 && v.ConfigMap.Name == "argocd-appset-gitlab-scm-tls-certs-cm" {
							volumeMatch = true
						}
					}
				}
			}
			Expect(volumeMatch).To(BeTrue())

			By("verifying volumemounts and volumes of Argo CD ApplicationSet controller")

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDepl), appsetDepl)).To(Succeed())

			Expect(appsetDepl.ObjectMeta.Labels["app.kubernetes.io/component"]).To(Equal("controller"))
			Expect(appsetDepl.ObjectMeta.Labels["app.kubernetes.io/managed-by"]).To(Equal("argocd"))
			Expect(appsetDepl.ObjectMeta.Labels["app.kubernetes.io/name"]).To(Equal("argocd-applicationset-controller"))
			Expect(appsetDepl.ObjectMeta.Labels["app.kubernetes.io/part-of"]).To(Equal("argocd"))

			Expect(appsetDepl.Spec.Template.Spec.Containers[0].VolumeMounts).To(Equal([]corev1.VolumeMount{
				{Name: "ssh-known-hosts", MountPath: "/app/config/ssh"},
				{Name: "tls-certs", MountPath: "/app/config/tls"},
				{Name: "gpg-keys", MountPath: "/app/config/gpg/source"},
				{Name: "gpg-keyring", MountPath: "/app/config/gpg/keys"},
				{Name: "tmp", MountPath: "/tmp"},
				{Name: "appset-gitlab-scm-tls-cert", MountPath: "/app/tls/scm/"},
			}))

			Expect(appsetDepl.Spec.Template.Spec.Volumes).To(Equal([]corev1.Volume{
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
					Name: "tmp", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "appset-gitlab-scm-tls-cert", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode:          ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{Name: "argocd-appset-gitlab-scm-tls-certs-cm"}},
					},
				},
			}))

		})

	})
})
