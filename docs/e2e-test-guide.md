# Argo CD Operator E2E Test Guide

E2E tests are written using [chainsaw](https://kyverno.github.io/chainsaw/latest/install/).

## Requirements

This test suite assumes that an Argo CD Operator is installed on the cluster or running locally using `ARGOCD_CLUSTER_CONFIG_NAMESPACES=argocd-e2e-cluster-config make install run`.

The system executing the tests must have following tools installed:

* `chainsaw`
* `oc` and `kubectl` client
* `jq` for parsing JSON data
* `curl`

There should be a `kubeconfig` pointing to your cluster, user should have full admin privileges (i.e. `kubeadm`).

!!! note 
    E2E tests utilize GNU Grep under the hood. Please make sure that you have the GNU compatible `grep` installed.

If you are on OSX you can install GNU compatible grep using the below command. The package is installed as `ggrep` by default. Please set this(ggrep) as an alias to `grep`.  

```sh
brew install grep
```

Use the below commands to install GNU compatible `grep` on OSX.

Also, note that the e2e tests for Redis HA mode require a cluster with at least three worker nodes.  A local three-worker node
cluster can be created using [k3d](https://k3d.io/)

## Running the tests

In any case, you should have set up your `kubeconfig` in such a way that your
default context points to the cluster you want to test. You can use the
`kubectl login ...` command to set this up for you.

## Run e2e tests

```sh
make e2e
```

## Run Operator locally and execute e2e tests

```sh
make all
```

### Running manual with chainsaw

```sh
chainsaw test ./tests/k8s
```

### Running single tests

Sometimes (e.g. when initially writing a test or troubleshooting an existing
one), you may want to run single test cases isolated. To do so, you can pass
the test folder to chainsaw, i.e.

```sh
chainsaw test ./tests/k8s/1-004_validate_namespace_scoped_install
```

The name of the test is the name of the directory containing its steps and
assertions.

If you are troubleshooting, you may want to prevent `chainsaw` from deleting the
test's namespace afterward. To do so, just pass the additional flag
`--skip-delete` to the above command.

## Writing new tests

### Name of the test

Each test comes in its own directory, containing all its test steps. The name
of the test is defined by the name of this directory.

The name of the test should be short, but expressive. The format for naming a
test is currently `<test ID>_<short description>`.

The `<test ID>` is the serial number of the test as defined in the Test Plan
document. The `<short description>` is exactly that, a short description of
what happens in the test.

Refer to the
[chainsaw documentation](https://kyverno.github.io/chainsaw/latest/writing-tests/)
for more information.

### Documentation

Documentation is important, even for tests. You can should provide inline
documentation in your YAML files (using comments) and a `README.md` in your
test case's directory. The `README.md` should provide some context for the
test case, e.g. what it tries to assert for under which circumstances. This
will help others in troubleshooting failing tests.

### Recipes

Generally, you should try to use `assert` and `errors` declaration whenever
possible and viable. For some cases, you may need to use custom scripts to
get the results you are looking for.

#### Scripts general

Scripts can be executed in a `Test` resource.

Your script probably will retrieve some information, and asserts it state. If
the assertion fails, the script should exit with a code > 0, and also print
some information why it failed, e.g.

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - script:
      content: |
        # Get some piece of information...
        if test "$result" != "expected"; then
          echo "Expectation failed, should 'expected', is '$result'"
          exit 1
        fi
```

Also, you may want to use `set -e` and `set -o pipefail` at the top of your
script to catch unexpected errors as test case failures, e.g.

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - script:
      content: |
        set -e
        set -o pipefail
        # rest of your script
```

#### Getting values of a resource's environment variables

YAML declarations used in `assert` or `errors` files unfortunately don't handle
arrays very well yet. You will always have to specify the complete expectation,
i.e. the complete array.

If you are just interested in a certain variable, and don't care about the rest,
you can use a script similar to the following using `jq`. E.g. to get the value
of a variable named `FOO` for the `argocd-server` deployment in the test's
namespace:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: example
spec:
  steps:
  - script:
      content: |
        val=$(kubectl get -n $NAMESPACE deployments argocd-server -o json \
          | jq -r '.spec.templates.spec.containers[0].env[]|select(.name=="FOO").value')
        if test "$val" != "bar"; then
          echo "Expectation failed for for env FOO in argocd-server: should 'bar', is '$val'"
          exit 1
        fi
```
