# argocd-operator E2E Tests

argocd-operator E2E tests are defined within the `tests/ginkgo` (as of this writing).

These tests are written with the Ginkgo/Gomega test framework, and were ported from previous Kuttl tests.

## Running tests

### A) Run tests against operator installed via OLM

The E2E tests can be run from the `Makefile` at the root of the argocd-operator repository.

```bash
# Run Sequential tests
make e2e-tests-sequential-ginkgo
# You can add 'SKIP_HA_TESTS=true' if you are on a cluster with <3 nodes
# Example: 'SKIP_HA_TESTS=true  make e2e-tests-sequential-ginkgo'

# Run Parallel tests (up to 5 tests will run at a time)
make e2e-tests-parallel-ginkgo
# As above, can add SKIP_HA_TESTS, if necessary.
```

### B) Run E2E tests against local operator (operator running via `make start-e2e` or `make run`)

```bash
# 1) Start operator locally
make start-e2e
# You can instead use 'make run', BUT, `make run` is missing the `ARGOCD_CLUSTER_CONFIG_NAMESPACES` env var

# 2) Start tests in LOCAL_RUN mode (this skips tests that require Subscription or CSVs)
LOCAL_RUN=true  make e2e-tests-sequential-ginkgo
# and/or
LOCAL_RUN=true  make e2e-tests-parallel-ginkgo
# Not all tests are supported when running locally. See 'Skip' messages for details.
```

### C) Run a specific test:

```bash
# 'make ginkgo' to download ginkgo, if needed
# Examples:
./bin/ginkgo -v -focus "1-002_validate_cluster_config"  -r ./tests/ginkgo/sequential
./bin/ginkgo -v -focus "1-099_validate_server_autoscale"  -r ./tests/ginkgo/parallel
```

## Configuring which tests run

Not all tests support all configurations:
* For example, if you are running argocd-operator via `make run`, this blocks any tests that require changes to `Subscription`. 
* Thus, when running locally, you can set `LOCAL_RUN=true` to skip those unsupported tests.

There are a few environment variables that can be set to configure which tests run. 


### If you are running the argocd-operator via `make start-e2e` or `make run` from your local machine

Some tests require the argocd-operator to be running on cluster (and/or installed via OLM). 

BUT, this is not true when you are running the operator on your local machine during the development process.

You can skip non-local-supported tests by setting `LOCAL_RUN=true`:
```bash
LOCAL_RUN=true  make e2e-tests-sequential-ginkgo
# and/or
LOCAL_RUN=true  make e2e-tests-sequential-parallel
```


### If you are running tests on a cluster with < 3 nodes:

Tests that verify operator HA (e.g. Redis HA) behaviour require a cluster with at least 3 nodes. If you are running on a cluster with less than 3 nodes, you can skip these tests by setting `SKIP_HA_TESTS=true`:
```bash
SKIP_HA_TESTS=true  make e2e-tests-sequential-ginkgo
```

### If you are testing an argocd-operator install that is running on K8s cluster, but that was NOT installed via Subscription (OLM)

In some cases, you may want to run the argocd-operator tests against an install that was NOT installed via OLM, but IS running on cluster. For example, via a plain `Deployment` in the gitops operator Namepsace.

For this, you may use the `NON_OLM` env var:
```bash
NON_OLM=true make e2e-tests-sequential-ginkgo
```

Note: If `LOCAL_RUN` is set, you do not need to set `NON_OLM` (it is assumed).


### You can specify multiple test env vars at the same time.

For example, if you are running operator via `make run`, on a non-HA cluster (<3 nodes):
```bash
SKIP_HA_TESTS=true LOCAL_RUN=true  make e2e-tests-sequential-ginkgo
```



## Test Code

argocd-operator E2E tests are defined within `tests/ginkgo`.

These tests are written with the [Ginkgo/Gomega test frameworks](https://github.com/onsi/ginkgo), and were ported from previous Kuttl tests.

### Tests are currently grouped as follows:
- `sequential`: Tests that are not safe to run in parallel with other tests.
    - A test is NOT safe to run in parallel with other tests if:
        - It modifies resources in operator namespaces
        - It modifies the operator `Subscription`
        - It modifies cluster-scoped resources, such as `ClusterRoles`/`ClusterRoleBindings`, or `Namespaces` that are shared between tests
        - More generally, if it writes to a K8s resource that is used by another test.
- `parallel`: Tests that are safe to run in parallel with other tests
    - A test is safe to run in parallel if it does not have any of the above problematic behaviours. 
    - It is fine for a parallel test to READ shared or cluster-scoped resources (such as resources in operator namespaces)
    - But a parallel test should NEVER write to resources that may be shared with other tests (`Subscriptions`, some cluster-scoped resources, etc.)

*Guidance*: Look at the list of restrictions for sequential. If your test is doing any of those things, it needs to run sequential. Otherwise parallel is fine.


### Test fixture:
- Utility functions for writing tests can be found within the `fixture/` folder.
- `fixture/fixture.go` contains utility functions that are generally useful to writing tests.
    - Most important are:
    - `EnsureParallelCleanSlate`: Should be called at the beginning of every parallel test.
    - `EnsureSequentialCleanSlate`: Should be called at the beginning of every sequential test.
- `fixture/(name of resource)` contains functions that are specific to working with a particular resource.
    - For example, if you wanted to wait for an `Application` CR to be Synced/Healthy, you would use the functions defined in `fixture/application`.
    - Likewise, if you want to check a `Deployment`, see `fixture/deployment`.
    - Fixtures exist for nearly all interesting resources
- The goal of this test fixture is to make it easy to write tests, and to ensure it is easy to understand and maintain existing tests.
- See existing k8s tests for usage examples.


## Writing new tests

Ginkgo tests are read from left to right. For example:
- `Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())`
    - Can be read as: Expect create of argo cd CR to suceeed.
- `Eventually(appControllerPod, "3m", "5s").Should(k8sFixture.ExistByName())`
    - Can be read as: Eventually the `(argo cd application controller pod)` should exist (within 3 minute, chekcing every 5 seconds.)
- `fixture.Update(argoCD, func(){ (...)})`
    - Can be reas ad: Update Argo CD CR using the given function


The E2E tests we use within this repo uses the standard controller-runtime k8s go API to interact with kubernetes (controller-runtime). This API is very familiar to anyone already writing go operator/controller code (such as developers of this project).

The best way to learn how to write a new test (or matcher/fixture), is just to copy an existing one!
- There are 150+ existing tests you can 'steal' from, which provide examples of nearly anything you could want.

### Standard patterns you can use

#### To verify a K8s resource has an expected status/spec:
- `fixture` packages
    - Fixture packages contain utility functions which exists for (nearly) all resources (described in detail elsewhere)
    - Most often, a function in a `fixture` will already exist for what you are looking for. 
        - For example, use `argocdFixture` to check if Argo CD is available:
            - `Eventually(argoCDbeta1, "5m", "5s").Should(argocdFixture.BeAvailable())`
    - Consider adding new functions to fixtures, so that tests can use them as well.
- If no fixture package function exists, just use a function that returns bool
	- Example: `1-005_validate_route_tls_test.go`

#### To create an object:
- `Expect(k8sClient.Create(ctx, (object))).Should(Succeed())`

#### To update an object, use `fixture.Update`
- `fixture.Update(object, func(){})` function
	- Test will automatically retry the update if update fails.
		- This avoids a common issue in k8s tests, where update fails which causes the test to fail.

#### To delete a k8s object
- `Expect(k8sClient.Delete(ctx, (object))).Should(Succeed())`
    - Where `(object)` is any k8s resource 


### Parallel vs Sequential

When to include a test in 'parallel' package, vs when to include a test in 'sequential' package? See elsewhere in this document for the exact criteria for when to include a test in parallel, and when to include it in sequential.

*General Guidance*: Look at the list of restrictions for sequential/parallel above. 
- If your test is performing any restricted behaviours, it needs to run sequential. Otherwise parallel is fine.
- For example: if your test modifies ANYTHING in operator Namespace, it's not safe to run in parallel. Include it in the `sequential` tests package.


#### When writing sequential tests, ensure you:

A) Call EnsureSequentialCleanSlate before each test:
```go
	BeforeEach(func() {
		fixture.EnsureSequentialCleanSlate()
	}
```

Unlike with parallel tests, you don't need to clean up namespace after each test. Sequential will automatically cleanup namespaces created via the `fixture.Create(...)Namespace` API. (But if you want to delete it using `defer`, it doesn't hurt).


#### When writing parallel tests, ensure you:

A) Call EnsureParallelCleanSlate before each test
```go
	BeforeEach(func() {
		fixture.EnsureParallelCleanSlate()
	})
```

B) Clean up any namespaces (or any cluster-scoped resources you created) using `defer`:
```go
// Create a namespace to use for the duration of the test, and then automatically clean it up after.
ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
defer cleanupFunc()
```

### General Tips
- DON'T ADD SLEEP STATEMENTS TO TESTS (unless it's absolutely necessary, but it rarely is!)
	- Use `Eventually`/`Consistently` with a condition, instead.
- Use `By("")` to document each step for what the test is doing.
	- This is very helpful for other team members that need to maintain your test after you wrote it.
	- Also all `By("")`s are included in test output as `Step: (...)`, which makes it easy to tell what the test is doing when the test is running.



## Translating from Kuttl to Ginkgo

### `01-create-or-update-resource.yaml`

Example:
In kuttl, this would create (or modify an existing) `ArgoCD` CR to have dex sso provider using openShiftOAuth.
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: argocd
spec:
  sso:
    provider: dex
    dex:
      openShiftOAuth: true
```

Equivalent in Ginkgo - to create:
```go
argocdObj := &argov1beta1api.ArgoCD{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "argocd",
		Namespace: "(namespace)",
	},
	Spec: argov1beta1api.ArgoCDSpec{
		SSO: &argov1beta1api.ArgoCDSSOSpec{
			Provider: argov1beta1api.SSOProviderTypeDex,
			Dex: &argov1beta1api.ArgoCDDexSpec{
				OpenShiftOAuth: true,
			},
		},
	},
}
Expect(k8sClient.Create(ctx, argocdObj)).To(Succeed())
```

Equivalent in Ginkgo - to update:
```go
argocdObj := &argov1beta1api.ArgoCD{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "argocd",
		Namespace: "(namespace)",
	},
}
argocdFixture.Update(argocdObj, func(ac *argov1beta1api.ArgoCD) {
	ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
		Provider: argov1beta1api.SSOProviderTypeDex,
		Dex: &argov1beta1api.ArgoCDDexSpec{
			OpenShiftOAuth: true,
		},
	}
})
```	

### `01-assert.yaml`

Example:
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
status:
  phase: Available
  sso: Running
```

The equivalent here is `Eventually`.

Equivalent in Ginkgo:
```go
Eventually(argoCDObject).Should(argocdFixture.BeAvailable())
Eventually(argoCDObject).Should(argocdFixture.HaveSSOStatus("Running"))
```

### `02-errors.yaml`

The close equivalent to an `errors.yaml` is Eventually with a Not, then a Consistently with a Not

Example:
```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
status:
  phase: Pending
  sso: Failed
```

Equivalent in Ginkgo:
```go
Eventually(argoCDObject).ShouldNot(argocdFixture.HavePhase("Pending"))
Consistently(argoCDObject).ShouldNot(argocdFixture.HavePhase("Pending"))

Eventually(argoCDObject).ShouldNot(argocdFixture.HaveSSOStatus("Failed"))
Consistently(argoCDObject).ShouldNot(argocdFixture.HaveSSOStatus("Failed"))
```





## Tips for debugging tests

### If you are debugging tests in CI
- If you are debugging a test failure, considering adding a call to the `fixture.OutputDebugOnFail()` function at the end of the test.
- `OutputDebugOnFail` will output helpful information when a test fails (such as namespace contents and operator pod logs)
- See existing test code for examples.


### If you are debugging tests locally
- Consider setting the `E2E_DEBUG_SKIP_CLEANUP` variable when debugging tests locally.
- The `E2E_DEBUG_SKIP_CLEANUP` environment variable will skip cleanup at the end of the test. 
    - The default E2E test behaviour is to clean up test resources at the end of the test. 
    - This is good when tests are succeeding, but when they are failing it can be helpful to look at the state of those K8s resources at the time of failure.
    - Those old tests resources WILL still be cleaned up when you next start the test again.
- This will allow you to `kubectl get` the test resource to see why the test failed. 

Example:
```bash
E2E_DEBUG_SKIP_CLEANUP=true ./bin/ginkgo -v -focus "1-099_validate_server_autoscale"  -r ./tests/ginkgo/parallel
```


## External Documentation

[**Ginkgo/Gomega docs**](https://onsi.github.io/gomega/): The Ginkgo/Gomega docs are great! they are very detailed, with lots of good examples. There are also plenty of other examples of Ginkgo/Gomega you can find via searching.

**Ask an LLM (Gemini/Cursor/etc)**: Ginkgo/gomega are popular enough that LLMs are able to answer questions and write code for them.
- For example, I performed the following Gemini Pro query, and got an excellent answer:
    - `With Ginkgo/Gomega (https://onsi.github.io/gomega) and Go lang, how do I create a matcher which checks whether a Kubernetes Deployment (via Deployment go object) has ready replicas of 1`
