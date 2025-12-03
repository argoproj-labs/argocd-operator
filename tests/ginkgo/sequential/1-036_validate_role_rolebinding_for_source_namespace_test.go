package sequential

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-036_validate_role_rolebinding_for_source_namespace", func() {

		var (
			ctx               context.Context
			k8sClient         client.Client
			argoNamespace     *corev1.Namespace
			cleanupArgoNSFunc func()

			defaultNSArgoCD *v1beta1.ArgoCD

			cleanupFunctions = []func(){} // we create various namespaces in this test, these functions will clean them up when the test is done
		)

		BeforeEach(func() {

			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {

			fixture.OutputDebugOnFail(argoNamespace)

			// Clean up argo cd instance created in test namespace first (before deleting the namespace itself)
			Expect(defaultNSArgoCD).ToNot(BeNil())
			err := k8sClient.Delete(ctx, defaultNSArgoCD)
			if err != nil && !apierrors.IsNotFound(err) {
				Expect(err).ToNot(HaveOccurred())
			}

			// Clean up namespaces created
			for _, namespaceCleanupFunction := range cleanupFunctions {
				namespaceCleanupFunction()
			}

		})

		It("verifies that ArgoCD CR '.spec.sourceNamespaces' field wildcard-matching matches and manages only namespaces which match the wildcard", func() {

			By("creating cluster-scoped namespace for Argo CD instance")
			argoNamespace, cleanupArgoNSFunc = fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			cleanupFunctions = append(cleanupFunctions, cleanupArgoNSFunc)

			By("creating test NS")
			testNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("creating Argo CD instance in argocd-e2e-cluster-config NS, with 'test' sourceNamespace only")
			defaultNSArgoCD = &v1beta1.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argoNamespace.Name,
				},
				Spec: v1beta1.ArgoCDSpec{
					SourceNamespaces: []string{
						"test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, defaultNSArgoCD)).To(Succeed())

			By("verifying Argo CD instance starts managing the namespace via managed-by-cluster-argocd label")
			Eventually(testNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			expectRoleAndRoleBindingValues := func(name string, ns string) {

				By("verifying Role/RoleBinding for " + name + " in " + ns + " namespace")
				roleTestNamespace := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
				}
				Eventually(roleTestNamespace).Should(k8sFixture.ExistByName())
				Expect(roleTestNamespace.Labels["app.kubernetes.io/managed-by"]).To(Equal("example-argocd"))
				Expect(roleTestNamespace.Labels["app.kubernetes.io/name"]).To(Equal("example-argocd"))
				Expect(roleTestNamespace.Labels["app.kubernetes.io/part-of"]).To(Equal("argocd"))

				rbTestNamespace := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
				}
				Eventually(rbTestNamespace).Should(k8sFixture.ExistByName())

				Expect(rbTestNamespace.RoleRef).To(Equal(rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Name:     name,
					Kind:     "Role",
				}))

				Expect(rbTestNamespace.Subjects).To(Equal([]rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "example-argocd-argocd-server",
						Namespace: argoNamespace.Name,
					},
					{
						Kind:      "ServiceAccount",
						Name:      "example-argocd-argocd-application-controller",
						Namespace: argoNamespace.Name,
					},
				}))

			}

			expectRoleAndRoleBindingValues("example-argocd_test", "test")

			By("creating test-1 and dev NS")
			test1NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-1")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			devNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("dev")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("updating Argo CD sourceNamespaces to test*, which should match test-1 and test but not dev")
			argocdFixture.Update(defaultNSArgoCD, func(ac *v1beta1.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{"test*"}
			})

			By("verifying test-1 NS becomes managed, and expected role/rolebindings exist in test* namespaces but not dev")
			Eventually(test1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			expectRoleAndRoleBindingValues("example-argocd_test", "test")

			expectRoleAndRoleBindingValues("example-argocd_test-1", "test-1")

			devRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_dev",
					Namespace: devNS.Name,
				},
			}
			devRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_dev",
					Namespace: devNS.Name,
				},
			}
			Consistently(devRole).Should(k8sFixture.NotExistByName())
			Consistently(devRoleBinding).Should(k8sFixture.NotExistByName())

			// ----

			By("creating a new test-2 NS")
			test2NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-2")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("verifying the test-2 namespace becomes managed by the argo cd instance, and has the expected role/rolebinding")
			Eventually(test2NS, "2m", "5s").Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			expectRoleAndRoleBindingValues("example-argocd_test-2", "test-2")

			// ----

			By("adding ALL namespaces ('*') to source namespaces of Argo CD instance")
			argocdFixture.Update(defaultNSArgoCD, func(ac *v1beta1.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{"*"}
			})

			By("verifying test, test-1, test-2, and dev are all managed and have the expected roles")
			Eventually(testNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(testNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			Eventually(test1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(test1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			Eventually(test2NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(test2NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			Eventually(devNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(devNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			expectRoleAndRoleBindingValues("example-argocd_test", "test")
			expectRoleAndRoleBindingValues("example-argocd_test-1", "test-1")
			expectRoleAndRoleBindingValues("example-argocd_test-2", "test-2")
			expectRoleAndRoleBindingValues("example-argocd_dev", "dev")

			// -----

			By("setting Argo CD instance sourceNamespaces to 'test-ns*' and 'dev-ns*'")
			argocdFixture.Update(defaultNSArgoCD, func(ac *v1beta1.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{"test-ns*", "dev-ns*"}
			})

			By("creating test-ns-1, dev-ns-1, and other-ns namespaces")
			test_ns_1NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-ns-1")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			dev_ns_1NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("dev-ns-1")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			other_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("other-ns")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("verifying test-ns-1 and dev-ns-1 are managed, but other-ns isn't")
			Eventually(test_ns_1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(test_ns_1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			Eventually(dev_ns_1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(dev_ns_1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			expectRoleAndRoleBindingValues("example-argocd_test-ns-1", "test-ns-1")
			expectRoleAndRoleBindingValues("example-argocd_dev-ns-1", "dev-ns-1")

			otherNSRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_other-ns",
					Namespace: other_NS.Name,
				},
			}
			otherNSRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_other-ns",
					Namespace: other_NS.Name,
				},
			}
			Consistently(otherNSRole).Should(k8sFixture.NotExistByName())
			Consistently(otherNSRoleBinding).Should(k8sFixture.NotExistByName())

			By("setting Argo CD instance sourceNamespaces to 'test-ns*'")

			argocdFixture.Update(defaultNSArgoCD, func(ac *v1beta1.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{"test-ns*"}
			})

			By("verifying dev-ns-1 eventually becomes unmanaged")
			Eventually(dev_ns_1NS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))
			Consistently(dev_ns_1NS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNamespace.Name))

			devns1Role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_dev-ns-1",
					Namespace: dev_ns_1NS.Name,
				},
			}
			devns1RoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_dev-ns-1",
					Namespace: dev_ns_1NS.Name,
				},
			}
			Consistently(devns1Role).Should(k8sFixture.NotExistByName())
			Consistently(devns1RoleBinding).Should(k8sFixture.NotExistByName())

		})

	})
})
