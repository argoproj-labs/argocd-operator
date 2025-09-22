package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all
	securityv1 "github.com/openshift/api/security/v1"

	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/onsi/gomega/format"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

const (
	// E2ETestLabelsKey and E2ETestLabelsValue are added to cluster-scoped resources (e.g. Namespaces) created by E2E tests (where possible). On startup (and before each test for sequential tests), any resources with this label will be deleted.
	E2ETestLabelsKey   = "app"
	E2ETestLabelsValue = "test-argo-app"
)

var NamespaceLabels = map[string]string{E2ETestLabelsKey: E2ETestLabelsValue}

func EnsureParallelCleanSlate() {

	// Increase the maximum length of debug output, for when tests fail
	format.MaxLength = 64 * 1024
	SetDefaultEventuallyTimeout(time.Second * 60)
	SetDefaultEventuallyPollingInterval(time.Second * 3)
	SetDefaultConsistentlyDuration(time.Second * 10)
	SetDefaultConsistentlyPollingInterval(time.Second * 1)

	// Unlike sequential clean slate, parallel clean slate cannot assume that there are no other tests running. This limits our ability to clean up old test artifacts.
}

// EnsureSequentialCleanSlate will clean up resources that were created during previous sequential tests
// - Deletes namespaces that were created by previous tests
// - Deletes other cluster-scoped resources that were created
// - Reverts changes made to Subscription CR
// - etc
func EnsureSequentialCleanSlate() {
	Expect(EnsureSequentialCleanSlateWithError()).To(Succeed())
}

func EnsureSequentialCleanSlateWithError() error {

	// With sequential tests, we are always safe to assume that there is no other test running. That allows us to clean up old test artifacts before new test starts.

	// Increase the maximum length of debug output, for when tests fail
	format.MaxLength = 64 * 1024
	SetDefaultEventuallyTimeout(time.Second * 60)
	SetDefaultEventuallyPollingInterval(time.Second * 3)
	SetDefaultConsistentlyDuration(time.Second * 10)
	SetDefaultConsistentlyPollingInterval(time.Second * 1)

	ctx := context.Background()
	k8sClient, _ := utils.GetE2ETestKubeClient()

	// Ensure namespaces created during test are deleted
	err := ensureTestNamespacesDeleted(ctx, k8sClient)
	if err != nil {
		return err
	}

	// Clean up old cluster-scoped role from 1-034
	_ = k8sClient.Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "custom-argocd-role"}})

	if RunningOnOpenShift() {
		// Delete 'restricted-dropcaps' which is created by at least one test
		scc := &securityv1.SecurityContextConstraints{
			ObjectMeta: metav1.ObjectMeta{
				Name: "restricted-dropcaps",
			},
		}
		if err := k8sClient.Delete(ctx, scc); err != nil {
			if !apierr.IsNotFound(err) {
				return err
			}
			// Otherwise, expected error if it doesn't exist.
		}
	}

	return nil
}

func CreateRandomE2ETestNamespace() *corev1.Namespace {

	randomVal := string(uuid.NewUUID())
	randomVal = randomVal[0:13] // Only use 14 characters of randomness. If we use more, then we start to hit limits on parts of code which limit # of characters to 63

	testNamespaceName := "gitops-e2e-test-" + randomVal

	ns := CreateNamespace(testNamespaceName)
	return ns
}

func CreateRandomE2ETestNamespaceWithCleanupFunc() (*corev1.Namespace, func()) {

	ns := CreateRandomE2ETestNamespace()
	return ns, nsDeletionFunc(ns)
}

// Create namespace for tests having a specific label for identification
// - If the namespace already exists, it will be deleted first
func CreateNamespace(name string) *corev1.Namespace {

	k8sClient, _ := utils.GetE2ETestKubeClient()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	// If the Namespace already exists, delete it first
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(ns), ns); err == nil {
		// Namespace exists, so delete it first
		Expect(deleteNamespaceAndVerify(context.Background(), ns.Name, k8sClient)).To(Succeed())
	}

	ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   name,
		Labels: NamespaceLabels,
	}}

	err := k8sClient.Create(context.Background(), ns)
	Expect(err).ToNot(HaveOccurred())

	return ns
}

func CreateNamespaceWithCleanupFunc(name string) (*corev1.Namespace, func()) {

	ns := CreateNamespace(name)
	return ns, nsDeletionFunc(ns)
}

// Create a namespace 'name' that is managed by another namespace 'managedByNamespace', via managed-by label.
func CreateManagedNamespace(name string, managedByNamespace string) *corev1.Namespace {
	k8sClient, _ := utils.GetE2ETestKubeClient()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

	// If the Namespace already exists, delete it first
	if err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(ns), ns); err == nil {
		// Namespace exists, so delete it first
		Expect(deleteNamespaceAndVerify(context.Background(), ns.Name, k8sClient)).To(Succeed())
	}

	ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			E2ETestLabelsKey:                E2ETestLabelsValue,
			"argocd.argoproj.io/managed-by": managedByNamespace,
		},
	}}

	Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())

	return ns

}

func CreateManagedNamespaceWithCleanupFunc(name string, managedByNamespace string) (*corev1.Namespace, func()) {
	ns := CreateManagedNamespace(name, managedByNamespace)
	return ns, nsDeletionFunc(ns)
}

// nsDeletionFunc is a convenience function that returns a function that deletes a namespace. This is used for Namespace cleanup by other functions.
func nsDeletionFunc(ns *corev1.Namespace) func() {

	return func() {
		DeleteNamespace(ns)
	}

}

func DeleteNamespace(ns *corev1.Namespace) {
	// If you are debugging an E2E test and want to prevent its namespace from being deleted when the test ends (so that you can examine the state of resources in the namespace) you can set E2E_DEBUG_SKIP_CLEANUP env var.
	if os.Getenv("E2E_DEBUG_SKIP_CLEANUP") != "" {
		GinkgoWriter.Println("Skipping namespace cleanup as E2E_DEBUG_SKIP_CLEANUP is set")
		return
	}

	k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
	Expect(err).ToNot(HaveOccurred())

	err = deleteNamespaceAndVerify(context.Background(), ns.Name, k8sClient)
	Expect(err).ToNot(HaveOccurred())

}

// EnvNonOLM checks if NON_OLM var is set; this variable is set when testing on GitOps operator that is not installed via OLM
func EnvNonOLM() bool {
	_, exists := os.LookupEnv("NON_OLM")
	return exists
}

func EnvLocalRun() bool {
	_, exists := os.LookupEnv("LOCAL_RUN")
	return exists
}

// EnvCI checks if CI env var is set; this variable is set when testing on GitOps Operator running via CI pipeline (and using an OLM Subscription)
func EnvCI() bool {
	_, exists := os.LookupEnv("CI")
	return exists
}

func WaitForAllDeploymentsInTheNamespaceToBeReady(ns string, k8sClient client.Client) {

	Eventually(func() bool {
		var deplList appsv1.DeploymentList

		if err := k8sClient.List(context.Background(), &deplList, client.InNamespace(ns)); err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		for _, depl := range deplList.Items {

			// If at least one of the Deployments has not been observed, wait and try again
			if depl.Generation != depl.Status.ObservedGeneration {
				return false
			}

			if int64(depl.Status.Replicas) != int64(depl.Status.ReadyReplicas) {
				return false
			}

		}

		// All Deployments in NS are reconciled and ready
		return true

	}, "3m", "1s").Should(BeTrue())

}

func WaitForAllStatefulSetsInTheNamespaceToBeReady(ns string, k8sClient client.Client) {

	Eventually(func() bool {
		var ssList appsv1.StatefulSetList

		if err := k8sClient.List(context.Background(), &ssList, client.InNamespace(ns)); err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		for _, ss := range ssList.Items {

			// If at least one of the StatefulSets has not been observed, wait and try again
			if ss.Generation != ss.Status.ObservedGeneration {
				return false
			}

			if int64(ss.Status.Replicas) != int64(ss.Status.ReadyReplicas) {
				return false
			}

		}

		// All StatefulSets in NS are reconciled and ready
		return true

	}, "3m", "1s").Should(BeTrue())

}

func WaitForAllPodsInTheNamespaceToBeReady(ns string, k8sClient client.Client) {

	Eventually(func() bool {
		var podList corev1.PodList

		if err := k8sClient.List(context.Background(), &podList, client.InNamespace(ns)); err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		for _, pod := range podList.Items {
			for _, containerStatus := range pod.Status.ContainerStatuses {

				if !containerStatus.Ready {
					GinkgoWriter.Println(pod.Name, "has container", containerStatus.Name, "which is not ready")
					return false
				}
			}

		}

		// All Pod in NS are ready
		return true

	}, "3m", "1s").Should(BeTrue())

}

// Delete all namespaces having a specific label used to identify namespaces that are created by e2e tests.
func ensureTestNamespacesDeleted(ctx context.Context, k8sClient client.Client) error {

	// fetch all namespaces having given label
	nsList, err := listE2ETestNamespaces(ctx, k8sClient)
	if err != nil {
		return fmt.Errorf("unable to delete test namespace: %w", err)
	}

	// delete selected namespaces
	for _, namespace := range nsList.Items {
		if err := deleteNamespaceAndVerify(ctx, namespace.Name, k8sClient); err != nil {
			return fmt.Errorf("unable to delete namespace '%s': %w", namespace.Name, err)
		}
	}
	return nil
}

// deleteNamespaceAndVerify deletes a namespace, and waits for it to be reported as deleted.
func deleteNamespaceAndVerify(ctx context.Context, namespaceParam string, k8sClient client.Client) error {

	GinkgoWriter.Println("Deleting Namespace", namespaceParam)

	// Delete the namespace:
	// - Issue a request to Delete the namespace
	// - Finally, we check if it has been deleted.
	if err := wait.PollUntilContextTimeout(ctx, time.Second*5, time.Minute*6, true, func(ctx context.Context) (done bool, err error) {
		// Delete the namespace, if it exists
		namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespaceParam,
			},
		}
		if err := k8sClient.Delete(ctx, &namespace); err != nil {
			if !apierr.IsNotFound(err) {
				GinkgoWriter.Printf("Unable to delete namespace '%s': %v\n", namespaceParam, err)
				return false, nil
			}
		}

		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(&namespace), &namespace); err != nil {
			if apierr.IsNotFound(err) {
				return true, nil
			} else {
				GinkgoWriter.Printf("Unable to Get namespace '%s': %v\n", namespaceParam, err)
				return false, nil
			}
		}

		return false, nil
	}); err != nil {
		return fmt.Errorf("namespace was never deleted, after delete was issued. '%s':%v", namespaceParam, err)
	}

	return nil
}

// Retrieve list of namespaces having a specific label used to identify namespaces that are created by e2e tests.
func listE2ETestNamespaces(ctx context.Context, k8sClient client.Client) (corev1.NamespaceList, error) {
	nsList := corev1.NamespaceList{}

	// set e2e label
	req, err := labels.NewRequirement(E2ETestLabelsKey, selection.Equals, []string{E2ETestLabelsValue})
	if err != nil {
		return nsList, fmt.Errorf("unable to set labels while fetching list of test namespace: %w", err)
	}

	// fetch all namespaces having given label
	err = k8sClient.List(ctx, &nsList, &client.ListOptions{LabelSelector: labels.NewSelector().Add(*req)})
	if err != nil {
		return nsList, fmt.Errorf("unable to fetch list of test namespace: %w", err)
	}
	return nsList, nil
}

// Update will keep trying to update object until it succeeds, or times out.
//
//nolint:unused
func updateWithoutConflict(obj client.Object, modify func(client.Object)) error {
	k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
	if err != nil {
		return err
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}

		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(context.Background(), obj)
	})

	return err
}

type testReportEntry struct {
	isOutputted bool
}

var testReportLock sync.Mutex
var testReportMap = map[string]testReportEntry{} // acquire testReportLock before reading/writing to this map, or any values within this map

// OutputDebugOnFail can be used to debug a failing test: it will output the operator logs and namespace info
// Parameters:
// - Will output debug information on namespaces specified as parameters.
// - Namespace parameter may be a string, *Namespace, or Namespace
func OutputDebugOnFail(namespaceParams ...any) {

	// Convert parameter to string of namespace name:
	// - You can specify Namespace, *Namespae, or string, and we will convert it to string namespace
	namespaces := []string{}
	for _, param := range namespaceParams {

		if param == nil {
			continue
		}

		if str, isString := (param).(string); isString {
			namespaces = append(namespaces, str)

		} else if nsPtr, isNsPtr := (param).(*corev1.Namespace); isNsPtr {
			namespaces = append(namespaces, nsPtr.Name)

		} else if ns, isNs := (param).(corev1.Namespace); isNs {
			namespaces = append(namespaces, ns.Name)

		} else {
			Fail(fmt.Sprintf("unrecognized parameter value: %v", param))
		}
	}

	csr := CurrentSpecReport()

	if !csr.Failed() || os.Getenv("SKIP_DEBUG_OUTPUT") == "true" {
		return
	}

	testName := strings.Join(csr.ContainerHierarchyTexts, " ")
	testReportLock.Lock()
	defer testReportLock.Unlock()
	debugOutput, exists := testReportMap[testName]

	if exists && debugOutput.isOutputted {
		// Skip output if we have already outputted once for this test
		return
	}

	testReportMap[testName] = testReportEntry{
		isOutputted: true,
	}

	for _, namespace := range namespaces {

		kubectlOutput, err := osFixture.ExecCommandWithOutputParam(false, "kubectl", "get", "all", "-n", namespace)
		if err != nil {
			GinkgoWriter.Println("unable to list", namespace, err, kubectlOutput)
			continue
		}

		GinkgoWriter.Println("")
		GinkgoWriter.Println("----------------------------------------------------------------")
		GinkgoWriter.Println("'kubectl get all -n", namespace+"' output:")
		GinkgoWriter.Println(kubectlOutput)
		GinkgoWriter.Println("----------------------------------------------------------------")

		kubectlOutput, err = osFixture.ExecCommandWithOutputParam(false, "kubectl", "get", "deployments", "-n", namespace, "-o", "yaml")
		if err != nil {
			GinkgoWriter.Println("unable to list", namespace, err, kubectlOutput)
			continue
		}

		GinkgoWriter.Println("")
		GinkgoWriter.Println("----------------------------------------------------------------")
		GinkgoWriter.Println("'kubectl get deployments -n " + namespace + " -o yaml")
		GinkgoWriter.Println(kubectlOutput)
		GinkgoWriter.Println("----------------------------------------------------------------")

	}

	kubectlOutput, err := osFixture.ExecCommandWithOutputParam(false, "kubectl", "get", "argocds", "-A", "-o", "yaml")
	if err != nil {
		GinkgoWriter.Println("unable to output all argo cd statuses", err, kubectlOutput)
	} else {
		GinkgoWriter.Println("")
		GinkgoWriter.Println("----------------------------------------------------------------")
		GinkgoWriter.Println("'kubectl get argocds -A -o yaml':")
		GinkgoWriter.Println(kubectlOutput)
		GinkgoWriter.Println("----------------------------------------------------------------")
	}

}

// EnsureRunningOnOpenShift should be called if a test requires OpenShift (for example, it uses Route CR).
func EnsureRunningOnOpenShift() {

	runningOnOpenShift := RunningOnOpenShift()

	if !runningOnOpenShift {
		Skip("This test requires the cluster to be OpenShift")
		return
	}

	Expect(runningOnOpenShift).To(BeTrueBecause("this test is marked as requiring an OpenShift cluster, and we have detected the cluster is OpenShift"))

}

// RunningOnOpenShift returns true if the cluster is an OpenShift cluster, false otherwise.
func RunningOnOpenShift() bool {
	k8sClient, _ := utils.GetE2ETestKubeClient()

	crdList := crdv1.CustomResourceDefinitionList{}
	Expect(k8sClient.List(context.Background(), &crdList)).To(Succeed())

	openshiftAPIsFound := 0
	for _, crd := range crdList.Items {
		if strings.Contains(crd.Spec.Group, "openshift.io") {
			openshiftAPIsFound++
		}
	}
	return openshiftAPIsFound > 5 // I picked 5 as an arbitrary number, could also just be 1
}

//nolint:unused
func outputPodLog(podSubstring string) {
	k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
	if err != nil {
		GinkgoWriter.Println(err)
		return
	}

	// List all pods on the cluster
	var podList corev1.PodList
	if err := k8sClient.List(context.Background(), &podList); err != nil {
		GinkgoWriter.Println(err)
		return
	}

	// Look specifically for operator pod
	matchingPods := []corev1.Pod{}
	for idx := range podList.Items {
		pod := podList.Items[idx]
		if strings.Contains(pod.Name, podSubstring) {
			matchingPods = append(matchingPods, pod)
		}
	}

	if len(matchingPods) == 0 {
		// This can happen when the operator is not running on the cluster
		GinkgoWriter.Println("DebugOutputOperatorLogs was called, but no pods were found.")
		return
	}

	if len(matchingPods) != 1 {
		GinkgoWriter.Println("unexpected number of operator pods", matchingPods)
		return
	}

	// Extract operator logs
	kubectlLogOutput, err := osFixture.ExecCommandWithOutputParam(false, "kubectl", "logs", "pod/"+matchingPods[0].Name, "manager", "-n", matchingPods[0].Namespace)
	if err != nil {
		GinkgoWriter.Println("unable to extract operator logs", err)
		return
	}

	// Output only the last 500 lines
	lines := strings.Split(kubectlLogOutput, "\n")

	startIndex := max(len(lines)-500, 0)

	GinkgoWriter.Println("")
	GinkgoWriter.Println("----------------------------------------------------------------")
	GinkgoWriter.Println("Log output from operator pod:")
	for _, line := range lines[startIndex:] {
		GinkgoWriter.Println(">", line)
	}
	GinkgoWriter.Println("----------------------------------------------------------------")

}
