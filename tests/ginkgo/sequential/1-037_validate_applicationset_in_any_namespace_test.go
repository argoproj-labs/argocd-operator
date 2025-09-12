package sequential

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	roleFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/role"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-037_validate_applicationset_in_any_namespace", func() {

		var (
			ctx              context.Context
			k8sClient        client.Client
			cleanupFunctions = []func(){} // we create various namespaces in this test, these functions will clean them up when the test is done

		)

		BeforeEach(func() {

			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {

			fixture.OutputDebugOnFail("appset-argocd", "appset-old-ns", "appset-new-ns")

			// Clean up namespaces created
			for _, namespaceCleanupFunction := range cleanupFunctions {
				namespaceCleanupFunction()
			}

		})

		It("verifying that ArgoCD CR '.spec.applicationset.sourcenamespaces' and '.spec.sourcenamespaces' correctly control role/rolebindings within the managed namespaces", func() {

			By("0) create namespaces: appset-argocd, appset-old-ns, appset-new-ns")

			appset_argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-argocd")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			appset_old_nsNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-old-ns")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			appset_new_nsNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-new-ns")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			// -----

			By("1) create Argo CD instance with no source namespaces")

			argoCD := &v1beta1.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: appset_argocdNS.Name,
				},
				Spec: v1beta1.ArgoCDSpec{
					ApplicationSet: &v1beta1.ArgoCDApplicationSet{
						SCMProviders: []string{
							"github.com",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			// Verifies that the role/rolebindings in the specified namespace are not managed by application controller or appset, in the given namespace
			expectRoleAndRoleBindingAndNamespaceToNotBeManaged := func(names []string, namespaceName string) {

				By(fmt.Sprintf("verifying that expected Role/Rolebindings %v exist in %s", names, namespaceName))
				for _, roleAndRoleBindingName := range names {

					By("verifying '" + roleAndRoleBindingName + "'")

					role := &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleAndRoleBindingName,
							Namespace: namespaceName,
						},
					}
					Eventually(role).Should(k8sFixture.NotExistByName())
					Consistently(role).Should(k8sFixture.NotExistByName())

					roleBinding := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleAndRoleBindingName,
							Namespace: namespaceName,
						},
					}
					Eventually(roleBinding).Should(k8sFixture.NotExistByName())
					Consistently(roleBinding).Should(k8sFixture.NotExistByName())

				}

				nsToCheck := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
					},
				}

				By("verifying that namespace" + namespaceName + " does not have label 'argocd.argoproj.io/applicationset-managed-by-cluster-argocd': 'appset-argocd'")
				Eventually(nsToCheck).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))
				Consistently(nsToCheck).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))

			}

			expectRoleAndRoleBindingAndNamespaceToNotBeManaged([]string{"example_appset-old-ns", "example-appset-argocd-applicationset"}, appset_old_nsNS.Name)

			// ----

			By("2) modifying ArgoCD to have one sourceNamespace: appset-old-ns")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}

			})

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			expectRoleAndRoleBindingAndNamespaceToNotBeManaged([]string{"example_appset-old-ns", "example-appset-argocd-applicationset"}, appset_old_nsNS.Name)

			// ----

			By("3) modifying ArgoCD to have 2 sourceNamespaces: appset-old-ns, appset-new-ns")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
					"appset-new-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}

			})

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			expectRoleAndRoleBindingAndNamespaceToNotBeManaged([]string{"example_appset-new-ns", "example-appset-argocd-applicationset"}, appset_new_nsNS.Name)
			expectRoleAndRoleBindingAndNamespaceToNotBeManaged([]string{"example_appset-old-ns", "example-appset-argocd-applicationset"}, appset_old_nsNS.Name)

			// ----

			By("4) Add a sourceNamespace of 'appset-new-ns' to ArgoCD CR")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.SourceNamespaces = []string{
					"appset-new-ns",
				}

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
					"appset-new-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}
			})

			By("verifying that Role in appset-new-ns has expected RBAC permissions: ability to modify applications, batch, and applicationsets")
			example_appset_new_nsRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example_appset-new-ns", Namespace: appset_new_nsNS.Name},
			}
			Eventually(example_appset_new_nsRole).Should(k8sFixture.ExistByName())

			Eventually(example_appset_new_nsRole).Should(roleFixture.HaveRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{"applications"},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{
						"jobs",
						"cronjobs",
						"cronjobs/finalizers",
					},
					Verbs: []string{"create", "update"},
				},
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{"applicationsets"},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
			}))

			By("verifying RoleBinding for argocd-server and argocd-application-controller exists in appset-new-ns namespace")
			example_appset_new_nsRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example_appset-new-ns",
					Namespace: appset_new_nsNS.Name,
				},
			}
			Eventually(example_appset_new_nsRoleBinding).Should(k8sFixture.ExistByName())
			Expect(example_appset_new_nsRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{

				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example_appset-new-ns",
			}))
			Expect(example_appset_new_nsRoleBinding.Subjects).To(Equal([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-server",
					Namespace: "appset-argocd",
				},
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-application-controller",
					Namespace: "appset-argocd",
				},
			}))

			example_appset_argocd_applicationsetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-appset-argocd-applicationset",
					Namespace: "appset-new-ns",
				},
			}
			Eventually(example_appset_argocd_applicationsetRole).Should(k8sFixture.ExistByName())

			example_appset_argocd_applicationsetRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-appset-argocd-applicationset",
					Namespace: "appset-new-ns",
				},
			}
			Eventually(example_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.ExistByName())

			By("verifying appset-new-ns namespace is managed as both a source namespace and an application set source namespace")

			Eventually(appset_new_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))

			Eventually(appset_new_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

			expectRoleAndRoleBindingAndNamespaceToNotBeManaged([]string{"example_appset-old-ns", "example-appset-argocd-applicationset"}, appset_old_nsNS.Name)

			// ----

			// appset resources should be created in appset-old-ns namespace as it is now a subset of sourceNamespaces i.e apps in any ns is enabled on appset-old-ns namespace
			By("5) Adds 'appset-old-ns' to spec.sourceNamespace")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.SourceNamespaces = []string{
					"appset-new-ns",
					"appset-old-ns",
				}

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
					"appset-new-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}
			})

			By("verifying that appset-old-ns gains Role/RoleBindings similar to appset-new-ns")
			example_appset_old_nsRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example_appset-old-ns",
					Namespace: appset_old_nsNS.Name,
				},
			}

			Eventually(example_appset_old_nsRole).Should(roleFixture.HaveRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{"applications"},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{
						"jobs",
						"cronjobs",
						"cronjobs/finalizers",
					},
					Verbs: []string{"create", "update"},
				},
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{
						"applicationsets",
					},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
			}))

			example_appset_old_nsRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example_appset-old-ns",
					Namespace: appset_old_nsNS.Name,
				},
			}

			Eventually(example_appset_old_nsRoleBinding).Should(k8sFixture.ExistByName())
			Expect(example_appset_old_nsRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{

				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example_appset-old-ns",
			}))
			Expect(example_appset_old_nsRoleBinding.Subjects).To(Equal([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-server",
					Namespace: "appset-argocd",
				},
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-application-controller",
					Namespace: "appset-argocd",
				},
			}))

			oldExample_appset_argocd_applicationsetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-appset-argocd-applicationset",
					Namespace: "appset-old-ns",
				},
			}
			Eventually(oldExample_appset_argocd_applicationsetRole).Should(k8sFixture.ExistByName())

			oldExample_appset_argocd_applicationsetRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-appset-argocd-applicationset",
					Namespace: "appset-old-ns",
				},
			}
			Eventually(oldExample_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.ExistByName())

			Eventually(appset_old_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))
			Consistently(appset_old_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

			Eventually(example_appset_new_nsRole).Should(k8sFixture.ExistByName())

			Eventually(example_appset_new_nsRole).Should(roleFixture.HaveRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{"applications"},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{
						"jobs",
						"cronjobs",
						"cronjobs/finalizers",
					},
					Verbs: []string{"create", "update"},
				},
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{
						"applicationsets",
					},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
			}))

			Eventually(example_appset_new_nsRoleBinding).Should(k8sFixture.ExistByName())
			Expect(example_appset_new_nsRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example_appset-new-ns",
			}))
			Expect(example_appset_new_nsRoleBinding.Subjects).To(Equal([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-server",
					Namespace: "appset-argocd",
				},
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-application-controller",
					Namespace: "appset-argocd",
				},
			}))

			Eventually(example_appset_argocd_applicationsetRole).Should(k8sFixture.ExistByName())
			Consistently(example_appset_argocd_applicationsetRole).Should(k8sFixture.ExistByName())

			Eventually(appset_new_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

			Eventually(appset_new_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))

			/// -------------

			// Appset resources should be removed and server role in target namespace should be update to remove appset permissions

			By("6) Remove 'appset-new-ns' from .spec.appliationSet.sourceNamespaces")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.SourceNamespaces = []string{
					"appset-new-ns",
					"appset-old-ns",
				}

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}
			})

			By("verifying that applicationsets has been removed from Role")
			Eventually(example_appset_new_nsRole).Should(k8sFixture.ExistByName())
			Eventually(example_appset_new_nsRole).Should(roleFixture.HaveRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{"applications"},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"update",
						"watch",
						"delete",
					},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{
						"jobs",
						"cronjobs",
						"cronjobs/finalizers",
					},
					Verbs: []string{"create", "update"},
				},
			}))

			By("verifying RoleBinding still has expected role and subjects")
			Eventually(example_appset_new_nsRoleBinding).Should(k8sFixture.ExistByName())
			Expect(example_appset_new_nsRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example_appset-new-ns",
			}))
			Expect(example_appset_new_nsRoleBinding.Subjects).To(Equal([]rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-server",
					Namespace: "appset-argocd",
				},
				{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-application-controller",
					Namespace: "appset-argocd",
				},
			}))

			By("verifying appset-new-ns namespace should still be managed-by-cluster-argocd")
			Eventually(appset_new_nsNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

			By("verifying appset-new-ns applicationset role/binding no longer exists in the namespace")
			Eventually(example_appset_argocd_applicationsetRole).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_argocd_applicationsetRole).Should(k8sFixture.NotExistByName())

			Eventually(example_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.NotExistByName())

			By("verifying appset-new-ns applicationset is not applicationset-managed-by Argo CD instance")
			Eventually(appset_new_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))
			Consistently(appset_new_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))

			// ---

			By("7) Remove all .spec.sourceNamespaces")

			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {

				ac.Spec.SourceNamespaces = []string{}

				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"appset-old-ns",
				}

				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}
			})

			By("verifying role/rolebinding no longer exists in any namespace")
			Eventually(example_appset_new_nsRole).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_new_nsRole).Should(k8sFixture.NotExistByName())
			Eventually(example_appset_new_nsRoleBinding).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_new_nsRoleBinding).Should(k8sFixture.NotExistByName())

			Eventually(example_appset_old_nsRole).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_old_nsRole).Should(k8sFixture.NotExistByName())
			Eventually(example_appset_old_nsRoleBinding).Should(k8sFixture.NotExistByName())
			Consistently(example_appset_old_nsRoleBinding).Should(k8sFixture.NotExistByName())

			Eventually(oldExample_appset_argocd_applicationsetRole).Should(k8sFixture.NotExistByName())
			Consistently(oldExample_appset_argocd_applicationsetRole).Should(k8sFixture.NotExistByName())
			Eventually(oldExample_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.NotExistByName())
			Consistently(oldExample_appset_argocd_applicationsetRoleBinding).Should(k8sFixture.NotExistByName())

			By("verifying applicationset-managed-by and managed-by are not set on any namespace")
			Eventually(appset_old_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))
			Consistently(appset_old_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", "appset-argocd"))

			Eventually(appset_old_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))
			Consistently(appset_old_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

			Eventually(appset_new_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))
			Consistently(appset_new_nsNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "appset-argocd"))

		})

	})
})
