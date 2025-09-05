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
	"strings"

	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deplFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	ssFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-057_validate_notifications", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that NotificationsConfiguration will configure Argo CD to send email on Application create", func() {

			By("creating simple namespace-scoped Argo CD instance with notifications enabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("creating Service/Deployment that will receive SMTP and write to file")

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "smtp4dev",
					Namespace: ns.Name,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "smtp4dev",
					},
					Ports: []corev1.ServicePort{
						{Name: "smtp", Protocol: corev1.ProtocolTCP, Port: 2525, TargetPort: intstr.FromInt(2525)},
						{Name: "http", Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt(80)},
					},
				},
			}
			Expect(k8sClient.Create(ctx, service)).To(Succeed())

			depl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "smtp4dev",
					Namespace: ns.Name,
					Labels: map[string]string{
						"app": "smtp4dev",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "smtp4dev"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "smtp4dev",
							},
						},
						Spec: corev1.PodSpec{
							Affinity: &corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
										NodeSelectorTerms: []corev1.NodeSelectorTerm{
											{MatchExpressions: []corev1.NodeSelectorRequirement{{
												Key:      "kubernetes.io/os",
												Operator: "In",
												Values: []string{
													"linux",
												},
											}}},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									Name: "smtp4dev",
									// quay.io/argoprojlabs/argocd-notifications-e2e-smtplistener:multiarch
									// Image: "quay.io/openshift-gitops-test/smtplistener:multiarch",
									Image: "quay.io/argoprojlabs/argocd-notifications-e2e-smtplistener:multiarch",
									Ports: []corev1.ContainerPort{
										{ContainerPort: int32(80)},
										{ContainerPort: int32(2525)},
									},
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, depl)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying all workloads are started")

			deploymentsShouldExist := []string{"argocd-redis", "argocd-server", "argocd-repo-server", "argocd-notifications-controller", "smtp4dev"}
			for _, depl := range deploymentsShouldExist {
				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: depl, Namespace: ns.Name}}
				Eventually(depl).Should(k8sFixture.ExistByName())
				Eventually(depl).Should(deplFixture.HaveReplicas(1))
				Eventually(depl, "3m", "5s").Should(deplFixture.HaveReadyReplicas(1), depl.Name+" was not ready")
			}

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(statefulSet).Should(ssFixture.HaveReplicas(1))
			Eventually(statefulSet, "3m", "5s").Should(ssFixture.HaveReadyReplicas(1))

			By("modifying NotificationsConfiguration to send email to smtp4dev pod")

			notificationsConfiguration := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: ns.Name,
				},
			}
			k8sFixture.Update(notificationsConfiguration, func(o client.Object) {
				ncObj, ok := o.(*argov1alpha1api.NotificationsConfiguration)
				Expect(ok).To(BeTrue())

				ncObj.Spec.Services = map[string]string{"service.email.gmail": "{host: smtp4dev, port: 2525, from: fake@email.com }"}

			})

			By("waiting for operator to reconcile our change to NotificationsConfiguration CR by checking argocd-notifications-cm has expected value")
			notificationConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-notifications-cm", Namespace: ns.Name}}
			Eventually(notificationConfigMap).Should(k8sFixture.ExistByName())
			Eventually(notificationConfigMap).Should(
				configmapFixture.HaveStringDataKeyValue("service.email.gmail", "{host: smtp4dev, port: 2525, from: fake@email.com }"))

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("creating Applications with notification annotation")
			app := &appv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app-3",
					Namespace: ns.Name,
					Annotations: map[string]string{
						"notifications.argoproj.io/subscribe.on-created.gmail": "jdfake@email.com",
					},
				},
				Spec: appv1alpha1.ApplicationSpec{
					Project: "default",
					Source: &appv1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						Path:           "test/examples/nginx",
						TargetRevision: "HEAD",
					},
					Destination: appv1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: ns.Name,
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("waiting for Argo CD to send an email to smtp4dev Pod indicating that the Application was created")

			podList := &corev1.PodList{}
			Expect(k8sClient.List(ctx, podList, &client.ListOptions{Namespace: ns.Name})).To(Succeed())
			var smtp4DevPod *corev1.Pod
			for idx := range podList.Items {
				item := podList.Items[idx]
				if item.Labels["app"] == "smtp4dev" {
					smtp4DevPod = &item
					break
				}
			}
			Expect(smtp4DevPod).ToNot(BeNil())

			Eventually(func() bool {

				out, err := osFixture.ExecCommand("kubectl", "-n", ns.Name, "exec", "--stdin", smtp4DevPod.Name, "--", "/bin/bash",
					"-c",
					"cat /tmp/*")

				GinkgoWriter.Println(out)

				if err != nil {
					GinkgoWriter.Println(err)
				}

				return strings.Contains(out, "Subject: Application my-app-3 has been created.")

			}, "2m", "5s").Should(BeTrue())

		})

	})
})
