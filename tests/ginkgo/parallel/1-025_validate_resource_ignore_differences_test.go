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

	Context("1-025_validate_resource_ignore_differences", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates setting Argo CD CR .spec.resourceIgnoreDifferences will cause the corresponding setting to be set on argocd-cm ConfigMap", func() {

			By("creating a basic Argo CD instance with .spec.resourceIgnoreDifferences set with custom values")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDYAML := `
metadata:
  name: example-argocd
spec:
  resourceIgnoreDifferences:
    all:
      jqPathExpressions:
          - xyz
          - abc
      jsonPointers:
          - xyz
          - abc
      managedFieldsManagers:
          - xyz
          - abc
    resourceIdentifiers:
      - group: apps
        kind: deployments
        customization:
          jqPathExpressions:
            - xyz
            - abc
          jsonPointers:
            - xyz
            - abc
          managedFieldsManagers:
            - xyz
            - abc
      - group: batch
        kind: jobs
        customization:
          jqPathExpressions:
            - xyz
            - abc
          jsonPointers:
            - xyz
            - abc
          managedFieldsManagers:
            - xyz
            - abc`

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
  resource.customizations.ignoreDifferences.all: |
    jqpathexpressions:
    - xyz
    - abc
    jsonpointers:
    - xyz
    - abc
    managedfieldsmanagers:
    - xyz
    - abc
  resource.customizations.ignoreDifferences.apps_deployments: |
    jqpathexpressions:
    - xyz
    - abc
    jsonpointers:
    - xyz
    - abc
    managedfieldsmanagers:
    - xyz
    - abc
  resource.customizations.ignoreDifferences.batch_jobs: |
    jqpathexpressions:
    - xyz
    - abc
    jsonpointers:
    - xyz
    - abc
    managedfieldsmanagers:
    - xyz
    - abc
`
			var expectedDataObj map[string]string
			Expect(yaml.Unmarshal([]byte(expectedDataFieldYaml), &expectedDataObj)).To(Succeed())

			for k, v := range expectedDataObj {
				Eventually(configMap).Should(configmapFixture.HaveStringDataKeyValue(k, v), "unable to locate '"+k+"': '"+v+"'")
			}

		})

	})
})
