package sequential

import (
	"context"
	"fmt"
	"strings"

	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	applicationFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	appprojectFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/appproject"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	clusterroleFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/clusterrole"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	roleFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/role"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

			fixture.OutputDebugOnFail("appset-argocd", "appset-old-ns", "appset-new-ns", "appset-namespace-scoped", "target-ns-1-037",
				"team-1", "team-2", "team-frontend", "team-backend", "team-3", "other-ns")

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

			By("verifying that the appset deployment does not contains 'applications in any namespace' parameter, since no source namespaces are specified")
			appsetDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-applicationset-controller",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())
			Expect(appsetDeployment).ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces", 0))

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

			By("verifying that the appset deployment does not contain 'applications in any namespace' parameter, because .spec.sourceNamespaces is not specified")
			appsetDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-applicationset-controller",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())
			Eventually(appsetDeployment).ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces", 0))

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

			By("verifying appset namespaces parameter exists, and it points to only the namespace specified in .spec.sourceNamespaces")
			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())
			Eventually(appsetDeployment).Should(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces appset-new-ns", 0))

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

			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())
			Eventually(appsetDeployment).Should(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces appset-new-ns,appset-old-ns", 0))

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

		It("verifies that ArgoCD sourcenamespaces resources are cleaned up automatically", func() {

			By("creating Argo CD namespace and appset source namespace")
			targetNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("target-ns-1-037")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			appset_argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-namespace-scoped")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("creating ArgoCD instance with target ns as a source NS, BUT note the ArgoCD instance is namespace-scoped")
			argoCD := &v1beta1.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: appset_argocdNS.Name,
				},
				Spec: v1beta1.ArgoCDSpec{
					ApplicationSet: &v1beta1.ArgoCDApplicationSet{
						SourceNamespaces: []string{targetNS.Name},
					},
					SourceNamespaces: []string{targetNS.Name},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD).Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			By("verifying that the appset deplomyent does not contain 'applications in any namespace' parameter")
			appsetDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-applicationset-controller",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())
			Expect(appsetDeployment).ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces", 0))

			By("first verify that the ClusterRole was not automatically created for the Argo CD instance")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:        argoCD.Name + "-" + argoCD.Namespace + "-" + common.ArgoCDApplicationSetControllerComponent,
					Annotations: common.DefaultAnnotations(argoCD.Name, argoCD.Namespace),
				},
			}
			Consistently(clusterRole).Should(k8sFixture.NotExistByName())

			By("creating ClusterRole and then ensuring it is automatically cleaned up")
			Expect(k8sClient.Create(ctx, clusterRole)).To(Succeed())
			Eventually(clusterRole).ShouldNot(k8sFixture.ExistByName())

			By("first verify that ClusterRoleBinding was not automatically created for the Argo CD instance")
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:        argoCD.Name + "-" + argoCD.Namespace + "-" + common.ArgoCDApplicationSetControllerComponent,
					Annotations: common.DefaultAnnotations(argoCD.Name, argoCD.Namespace),
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "sa",
						Namespace: argoCD.Namespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     clusterRole.Name,
				},
			}
			Consistently(clusterRoleBinding).Should(k8sFixture.NotExistByName())
			By("creating ClusterRoleBinding and then ensuring it is automatically cleaned up")
			Expect(k8sClient.Create(ctx, clusterRoleBinding)).To(Succeed())
			Eventually(clusterRoleBinding).ShouldNot(k8sFixture.ExistByName())

			By("first verifying that Role does not exist in namespace specified in appset sourceNamespaces field")
			roleInTargetNS := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-applicationset", argoCD.Name, argoCD.Namespace),
					Namespace: targetNS.Name,
				},
			}
			Consistently(roleInTargetNS).Should(k8sFixture.NotExistByName())
			By("creating Role in source NS and verifying it is not cleaned up (yet)")
			Expect(k8sClient.Create(ctx, roleInTargetNS)).To(Succeed())
			Consistently(roleInTargetNS).Should(k8sFixture.ExistByName())

			By("verifying that there exist no rolebindings that point to the namespace-scoped argocd instance namespace")
			Consistently(func() bool {
				var roleBindings rbacv1.RoleBindingList
				if err := k8sClient.List(ctx, &roleBindings, client.InNamespace(targetNS.Name)); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				for _, crb := range roleBindings.Items {
					for _, subject := range crb.Subjects {
						if subject.Namespace == argoCD.Namespace {
							GinkgoWriter.Println("detected an RB that pointed to namespace scoped ArgoCD instance. This shouldn't happen:", crb.Name)
							return false
						}
					}
				}
				return true

			}).Should(BeTrue())

			By("first verifying that RoleBinding does not exist in source namespace")
			roleBindingInTargetNS := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-applicationset", argoCD.Name, argoCD.Namespace),
					Namespace: targetNS.Name,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      "ServiceAccount",
						Name:      "sa",
						Namespace: argoCD.Namespace,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     roleInTargetNS.Name,
				},
			}
			Consistently(roleBindingInTargetNS).Should(k8sFixture.NotExistByName())
			By("creating RoleBinding in source NS and verifying it is not cleaned up (yet)")
			Expect(k8sClient.Create(ctx, roleBindingInTargetNS)).To(Succeed())
			Consistently(roleBindingInTargetNS).Should(k8sFixture.ExistByName())

			By("adding ArgoCDApplicationSetManagedByClusterArgoCDLabel label to target NS")
			namespaceFixture.Update(targetNS, func(n *corev1.Namespace) {
				if n.Labels == nil {
					n.Labels = map[string]string{}
				}
				n.Labels[common.ArgoCDApplicationSetManagedByClusterArgoCDLabel] = argoCD.Namespace
			})

			By("verifying the label is automatically removed")
			Eventually(targetNS).Should(k8sFixture.NotHaveLabelWithValue(common.ArgoCDApplicationSetManagedByClusterArgoCDLabel, argoCD.Namespace))

			By("verifying that the roles/rolebindings we created in the previous steps are now automatically cleaned up, because the namespace had the ArgoCDApplicationSetManagedByClusterArgoCDLabel")
			Eventually(roleBindingInTargetNS).Should(k8sFixture.NotExistByName())
			Eventually(roleInTargetNS).Should(k8sFixture.NotExistByName())
		})

		It("verifies that wildcard patterns in .spec.applicationSet.sourceNamespaces correctly match and manage multiple namespaces", func() {

			By("0) create namespaces: appset-argocd, team-1, team-2, team-frontend, team-backend, other-ns")

			appset_wildcard_argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-argocd")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			team1NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("team-1")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			team2NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("team-2")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			teamFrontendNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("team-frontend")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			teamBackendNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("team-backend")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			otherNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("other-ns")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			// -----

			By("1) create Argo CD instance with wildcard pattern 'team-*' in both sourceNamespaces and applicationSet.sourceNamespaces")

			argoCD := &v1beta1.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example",
					Namespace: appset_wildcard_argocdNS.Name,
				},
				Spec: v1beta1.ArgoCDSpec{
					SourceNamespaces: []string{
						"team-*",
					},
					ApplicationSet: &v1beta1.ArgoCDApplicationSet{
						SourceNamespaces: []string{
							"team-*",
						},
						SCMProviders: []string{
							"github.com",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			By("2) verifying that the appset deployment contains all matching namespaces in the command")
			appsetDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-applicationset-controller",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(appsetDeployment).Should(k8sFixture.ExistByName())

			// Verify that all team-* namespaces are included (order may vary)
			Eventually(appsetDeployment).Should(deploymentFixture.HaveContainerCommandSubstring("--applicationset-namespaces", 0))
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDeployment), appsetDeployment); err != nil {
					return false
				}
				if len(appsetDeployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := appsetDeployment.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				if strings.Contains(cmdStr, "--applicationset-namespaces") {
					// Check that all team-* namespaces are present
					return strings.Contains(cmdStr, "team-1") &&
						strings.Contains(cmdStr, "team-2") &&
						strings.Contains(cmdStr, "team-frontend") &&
						strings.Contains(cmdStr, "team-backend")
				}
				return false
			}).Should(BeTrue())

			By("3) verifying that Role and RoleBinding are created in all matching team-* namespaces")
			verifyAppSetResourcesInNamespace := func(namespaceName string) {
				appsetRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wildcard-example-appset-argocd-applicationset",
						Namespace: namespaceName,
					},
				}
				Eventually(appsetRole).Should(k8sFixture.ExistByName())

				appsetRoleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wildcard-example-appset-argocd-applicationset",
						Namespace: namespaceName,
					},
				}
				Eventually(appsetRoleBinding).Should(k8sFixture.ExistByName())
				Expect(appsetRoleBinding.RoleRef).To(Equal(rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     "wildcard-example-appset-argocd-applicationset",
				}))
				Expect(appsetRoleBinding.Subjects).To(ContainElement(rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "wildcard-example-applicationset-controller",
					Namespace: appset_wildcard_argocdNS.Name,
				}))
			}

			verifyAppSetResourcesInNamespace(team1NS.Name)
			verifyAppSetResourcesInNamespace(team2NS.Name)
			verifyAppSetResourcesInNamespace(teamFrontendNS.Name)
			verifyAppSetResourcesInNamespace(teamBackendNS.Name)

			By("4) verifying that namespace labels are set correctly for all matching namespaces")
			Eventually(team1NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))
			Eventually(team2NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))
			Eventually(teamFrontendNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))
			Eventually(teamBackendNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))

			By("5) verifying that non-matching namespace (other-ns) does NOT have appset resources")
			otherNSAppSetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: otherNS.Name,
				},
			}
			Consistently(otherNSAppSetRole).Should(k8sFixture.NotExistByName())

			otherNSAppSetRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: otherNS.Name,
				},
			}
			Consistently(otherNSAppSetRoleBinding).Should(k8sFixture.NotExistByName())

			Consistently(otherNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))

			By("6) creating a new namespace that matches the pattern and verifying it gets resources automatically")
			team3NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("team-3")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			// Wait for reconciliation to pick up the new namespace
			Eventually(func() bool {
				appsetRole := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wildcard-example-appset-argocd-applicationset",
						Namespace: team3NS.Name,
					},
				}
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetRole), appsetRole) == nil
			}, "2m", "5s").Should(BeTrue())

			verifyAppSetResourcesInNamespace(team3NS.Name)
			Eventually(team3NS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))

			By("7) updating ArgoCD to use a more specific pattern 'team-*' -> 'team-1' and verifying cleanup")
			argocdFixture.Update(argoCD, func(ac *v1beta1.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{
					"team-1",
				}
				ac.Spec.ApplicationSet.SourceNamespaces = []string{
					"team-1",
				}
				ac.Spec.ApplicationSet.SCMProviders = []string{
					"github.com",
				}
			})

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("8) verifying that team-1 still has resources")
			team1AppSetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: team1NS.Name,
				},
			}
			Eventually(team1AppSetRole).Should(k8sFixture.ExistByName())

			By("9) verifying that other team-* namespaces have resources cleaned up")
			team2AppSetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: team2NS.Name,
				},
			}
			Eventually(team2AppSetRole).Should(k8sFixture.NotExistByName())
			Consistently(team2AppSetRole).Should(k8sFixture.NotExistByName())

			team3AppSetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: team3NS.Name,
				},
			}
			Eventually(team3AppSetRole).Should(k8sFixture.NotExistByName())
			Consistently(team3AppSetRole).Should(k8sFixture.NotExistByName())

			teamFrontendAppSetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-example-appset-argocd-applicationset",
					Namespace: teamFrontendNS.Name,
				},
			}
			Eventually(teamFrontendAppSetRole).Should(k8sFixture.NotExistByName())
			Consistently(teamFrontendAppSetRole).Should(k8sFixture.NotExistByName())

			By("10) verifying that labels are removed from namespaces that no longer match")
			Eventually(team2NS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))
			Eventually(team3NS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))
			Eventually(teamFrontendNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/applicationset-managed-by-cluster-argocd", appset_wildcard_argocdNS.Name))

			By("11) verifying deployment command only includes team-1")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDeployment), appsetDeployment); err != nil {
					return false
				}
				if len(appsetDeployment.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := appsetDeployment.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				if strings.Contains(cmdStr, "--applicationset-namespaces") {
					return strings.Contains(cmdStr, "team-1") &&
						!strings.Contains(cmdStr, "team-2") &&
						!strings.Contains(cmdStr, "team-3") &&
						!strings.Contains(cmdStr, "team-frontend")
				}
				return false
			}).Should(BeTrue())

		})

		It("verifies ApplicationSet clusterrole rules and creates appset/app in another namespace", func() {

			By("creating Argo CD namespace and target source namespace")
			argoNamespace, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-argocd-clusterrole")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			targetNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("appset-target-ns")
			cleanupFunctions = append(cleanupFunctions, cleanupFunc)

			By("creating Argo CD instance with source namespaces")
			argoCD := &v1beta1.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "appset-example",
					Namespace: argoNamespace.Name,
				},
				Spec: v1beta1.ArgoCDSpec{
					SourceNamespaces: []string{
						targetNS.Name,
					},
					ApplicationSet: &v1beta1.ArgoCDApplicationSet{
						SourceNamespaces: []string{
							targetNS.Name,
						},
						SCMProviders: []string{
							"github.com",
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			appProject := &appv1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(appProject).Should(k8sFixture.ExistByName())
			appprojectFixture.Update(appProject, func(appProject *appv1alpha1.AppProject) {
				appProject.Spec.SourceNamespaces = append(appProject.Spec.SourceNamespaces, targetNS.Name)
			})

			By("verifying ApplicationSet controller ClusterRole has expected rules")
			appsetClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: argoCD.Name + "-" + argoCD.Namespace + "-" + common.ArgoCDApplicationSetControllerComponent,
				},
			}
			Eventually(appsetClusterRole).Should(k8sFixture.ExistByName())
			Eventually(appsetClusterRole).Should(clusterroleFixture.HaveRules([]rbacv1.PolicyRule{
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{
						"applications",
						"applicationsets",
						"applicationsets/finalizers",
					},
					Verbs: []string{
						"create",
						"delete",
						"get",
						"list",
						"patch",
						"update",
						"watch",
					},
				},
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{
						"appprojects",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
				{
					APIGroups: []string{"argoproj.io"},
					Resources: []string{
						"applicationsets/status",
					},
					Verbs: []string{
						"get",
						"patch",
						"update",
					},
				},
				{
					APIGroups: []string{""},
					Resources: []string{
						"events",
					},
					Verbs: []string{
						"create",
						"get",
						"list",
						"patch",
						"watch",
					},
				},
				{
					APIGroups: []string{""},
					Resources: []string{
						"secrets",
						"configmaps",
					},
					Verbs: []string{
						"get",
						"list",
						"watch",
					},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{
						"leases",
					},
					Verbs: []string{
						"create",
					},
				},
				{
					APIGroups: []string{"coordination.k8s.io"},
					Resources: []string{
						"leases",
					},
					Verbs: []string{
						"get",
						"update",
						"create",
					},
					ResourceNames: []string{
						"58ac56fa.applicationsets.argoproj.io",
					},
				},
			}))

			By("creating an ApplicationSet in the target namespace")
			appset := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "argoproj.io/v1alpha1",
					"kind":       "ApplicationSet",
					"metadata": map[string]interface{}{
						"name":      "guestbook-appset",
						"namespace": targetNS.Name,
					},
					"spec": map[string]interface{}{
						"generators": []interface{}{
							map[string]interface{}{
								"list": map[string]interface{}{
									"elements": []interface{}{
										map[string]interface{}{
											"name": "guestbook",
										},
									},
								},
							},
						},
						"template": map[string]interface{}{
							"metadata": map[string]interface{}{
								"name": "{{name}}",
							},
							"spec": map[string]interface{}{
								"project": "default",
								"source": map[string]interface{}{
									"repoURL":        "https://github.com/argoproj/argocd-example-apps.git",
									"targetRevision": "HEAD",
									"path":           "guestbook",
								},
								"destination": map[string]interface{}{
									"server":    "https://kubernetes.default.svc",
									"namespace": targetNS.Name,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, appset)).To(Succeed())
			Eventually(appset).Should(k8sFixture.ExistByName())

			By("verifying ApplicationSet generates Application in target namespace")
			generatedApp := &appv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guestbook",
					Namespace: targetNS.Name,
				},
			}
			Eventually(generatedApp, "5m", "10s").Should(k8sFixture.ExistByName())
			Eventually(generatedApp).Should(applicationFixture.HaveHealthStatusCode(health.HealthStatusMissing))
			Eventually(generatedApp).Should(applicationFixture.HaveSyncStatusCode(appv1alpha1.SyncStatusCodeOutOfSync))
			By("Cleaning up the ApplicationSet")
			Expect(k8sClient.Delete(ctx, appset)).To(Succeed())
			Eventually(appset).Should(k8sFixture.NotExistByName())
		})

	})
})
