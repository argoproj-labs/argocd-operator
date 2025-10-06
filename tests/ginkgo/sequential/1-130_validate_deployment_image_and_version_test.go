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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

const (
	controllerManagerName      = "argocd-operator-controller-manager"
	controllerManagerNamespace = "argocd-operator-system"
	argocdInstanceName         = "argocd"
	argocdNamespace            = "argocd"
	defaultImage               = "argoproj/argocd@sha123456"
	expectedDefaultImage       = "argoproj/argocd:sha123456"
)

// --- Local Helpers (kept inline for reuse in this test) ---

func ensureEnv(ctx context.Context, c client.Client, d *appsv1.Deployment) (string, bool) {
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "ARGOCD_IMAGE" {
			return env.Value, false
		}
	}
	d.Spec.Template.Spec.Containers[0].Env =
		append(d.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "ARGOCD_IMAGE", Value: defaultImage})
	Expect(c.Update(ctx, d)).To(Succeed())
	waitForDeploymentReady(ctx, c, d)
	return defaultImage, true
}

func removeEnv(ctx context.Context, c client.Client, d *appsv1.Deployment) {
	var newEnvs []corev1.EnvVar
	for _, env := range d.Spec.Template.Spec.Containers[0].Env {
		if env.Name != "ARGOCD_IMAGE" {
			newEnvs = append(newEnvs, env)
		}
	}
	d.Spec.Template.Spec.Containers[0].Env = newEnvs
	Expect(c.Update(ctx, d)).To(Succeed())
	waitForDeploymentReady(ctx, c, d)
}

func waitForDeploymentReady(ctx context.Context, c client.Client, d *appsv1.Deployment) {
	Eventually(func() bool {
		err := c.Get(ctx, types.NamespacedName{Name: d.Name, Namespace: d.Namespace}, d)
		return err == nil && d.Status.ReadyReplicas == 1
	}, "2m", "5s").Should(BeTrue())
}

func waitForDeployments(ctx context.Context, c client.Client, ns string, min int) *appsv1.DeploymentList {
	deploys := &appsv1.DeploymentList{}
	Eventually(func() error {
		if err := c.List(ctx, deploys, client.InNamespace(ns)); err != nil {
			return err
		}
		if len(deploys.Items) < min {
			return fmt.Errorf("expected at least %d deployments, got %d", min, len(deploys.Items))
		}
		return nil
	}, "2m", "5s").Should(Succeed())
	return deploys
}

func assertDeploymentImages(deploys *appsv1.DeploymentList, expected map[string]string) {
	for _, d := range deploys.Items {
		if exp, ok := expected[d.Name]; ok {
			Expect(d.Spec.Template.Spec.Containers).To(HaveLen(1))
			got := d.Spec.Template.Spec.Containers[0].Image
			Expect(got).To(Equal(exp), fmt.Sprintf("deployment %s has %s, expected %s", d.Name, got, exp))
		}
	}
}

func assertStatefulSetImage(ctx context.Context, c client.Client, ns, name, expected string) {
	sts := &appsv1.StatefulSet{}
	Expect(c.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, sts)).To(Succeed())
	Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
	got := sts.Spec.Template.Spec.Containers[0].Image
	Expect(got).To(Equal(expected), fmt.Sprintf("statefulset %s has %s, expected %s", name, got, expected))
}

// --- Test ---

var _ = Describe("Validate deployment image and version", func() {
	Context("1-130_validate_deployment_image_and_version", func() {
		var (
			c   client.Client
			ctx context.Context
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			c, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates image resolution order", func() {
			By("ensuring operator env variable ARGOCD_IMAGE is set")
			deploy := &appsv1.Deployment{}
			Expect(c.Get(ctx, types.NamespacedName{Name: controllerManagerName, Namespace: controllerManagerNamespace}, deploy)).To(Succeed())
			_, added := ensureEnv(ctx, c, deploy)

			By("creating namespace-scoped ArgoCD without image/version")
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: argocdNamespace}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			enabled := true
			argo := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: argocdInstanceName, Namespace: argocdNamespace},
				Spec:       argov1beta1api.ArgoCDSpec{ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{Enabled: &enabled}},
			}
			Expect(c.Create(ctx, argo)).To(Succeed())
			Eventually(func() error {
				return c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: ns.Name}, &argov1beta1api.ArgoCD{})
			}, "2m", "5s").Should(Succeed())

			defer func() {
				if added {
					By("removing operator env var ARGOCD_IMAGE")
					Expect(c.Get(ctx, types.NamespacedName{Name: controllerManagerName, Namespace: controllerManagerNamespace}, deploy)).To(Succeed())
					removeEnv(ctx, c, deploy)
				}
				Expect(c.Delete(ctx, argo)).To(Succeed())
				Expect(c.Delete(ctx, ns)).To(Succeed())
			}()

			checkImages := func(expected map[string]string, stsExpected string) {
				deploys := waitForDeployments(ctx, c, ns.Name, 4)
				assertDeploymentImages(deploys, expected)
				assertStatefulSetImage(ctx, c, ns.Name, "argocd-application-controller", stsExpected)
			}

			By("verifying deployments/statefulset use env image")
			checkImages(map[string]string{
				"argocd-repo-server":               expectedDefaultImage,
				"argocd-server":                    expectedDefaultImage,
				"argocd-applicationset-controller": expectedDefaultImage,
			}, expectedDefaultImage)

			By("updating spec.version")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: ns.Name}, argo)).To(Succeed())
			argo.Spec.Version = "v0.0.1"
			Expect(c.Update(ctx, argo)).To(Succeed())
			time.Sleep(30 * time.Second)
			checkImages(map[string]string{
				"argocd-repo-server":               "argoproj/argocd:v0.0.1",
				"argocd-server":                    "argoproj/argocd:v0.0.1",
				"argocd-applicationset-controller": "argoproj/argocd:v0.0.1",
			}, "argoproj/argocd:v0.0.1")

			By("updating spec.version and spec.image")
			Expect(c.Get(ctx, types.NamespacedName{Name: argocdInstanceName, Namespace: ns.Name}, argo)).To(Succeed())
			argo.Spec.Image, argo.Spec.Version = "argoprpj/sampletestimage", "v0.0.1"
			Expect(c.Update(ctx, argo)).To(Succeed())
			time.Sleep(30 * time.Second)
			checkImages(map[string]string{
				"argocd-repo-server":               "argoprpj/sampletestimage:v0.0.1",
				"argocd-server":                    "argoprpj/sampletestimage:v0.0.1",
				"argocd-applicationset-controller": "argoprpj/sampletestimage:v0.0.1",
			}, "argoprpj/sampletestimage:v0.0.1")
		})
	})
})
