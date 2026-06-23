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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-026_validate_resource_action", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates setting Argo CD CR .spec.resourceActions will cause the corresponding setting to be set on argocd-cm ConfigMap", func() {

			By("creating a basic Argo CD instance with .spec.resourceActions set with custom values")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDYAML := `
metadata:
  name: example-argocd
spec:
  resourceActions:
    - group: apps
      kind: Deployment
      action: |
        discovery.lua: |
        actions = {}
        actions["restart"] = {}
        return actions
        definitions:
        - name: restart
          # Lua Script to modify the obj
          action.lua: |
            local os = require("os")
            if obj.spec.template.metadata == nil then
                obj.spec.template.metadata = {}
            end
            if obj.spec.template.metadata.annotations == nil then
                obj.spec.template.metadata.annotations = {}
            end
            obj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
            return obj`

			// We unmarshal YAML into ArgoCD CR, so that we don't have to convert it into Go structs (it would be painful)
			argoCD := &argov1beta1api.ArgoCD{}
			Expect(yaml.UnmarshalStrict([]byte(argoCDYAML), &argoCD)).To(Succeed())
			argoCD.Namespace = ns.Name
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for each of the .data fields of argocd-cm ConfigMap to have expected value")
			configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name}}
			Eventually(configMap).Should(k8sFixture.ExistByName())

			expectedDataFieldYaml := `
  resource.customizations.actions.apps_Deployment: |
    discovery.lua: |
    actions = {}
    actions["restart"] = {}
    return actions
    definitions:
    - name: restart
      # Lua Script to modify the obj
      action.lua: |
        local os = require("os")
        if obj.spec.template.metadata == nil then
            obj.spec.template.metadata = {}
        end
        if obj.spec.template.metadata.annotations == nil then
            obj.spec.template.metadata.annotations = {}
        end
        obj.spec.template.metadata.annotations["kubectl.kubernetes.io/restartedAt"] = os.date("!%Y-%m-%dT%XZ")
        return obj`
			var expectedDataObj map[string]string
			Expect(yaml.Unmarshal([]byte(expectedDataFieldYaml), &expectedDataObj)).To(Succeed())

			for k, v := range expectedDataObj {
				Eventually(configMap).Should(configmapFixture.HaveStringDataKeyValue(k, v), "unable to locate '"+k+"': '"+v+"'")
			}

		})

	})
})
