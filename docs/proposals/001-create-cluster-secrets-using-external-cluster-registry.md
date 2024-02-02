# Auto create and manage Argo CD cluster secrets using an external cluster registry

## Summary

Add the ability to automatically create Argo CD 
[cluster secrets](https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters) 
using an external cluster registry.

## Motivation

There are third party projects that have a concept of cluster registry or cluster inventory.
These clusters are usually represented by a CRD.
For example in [Open Cluster Management](https://open-cluster-management.io/),
the API is [ManagedCluster](https://open-cluster-management.io/concepts/managedcluster/).
When users of these projects want to use Argo CD, they still have to manually add and manage clusters.
This proposal helps improve the user experience of Argo CD for those projects
by providing a way for the operator to look at an existing third party cluster registry
and auto create and manage Argo CD cluster secrets.

As stated on the [website](https://argocd-operator.readthedocs.io/en/latest/#overview),
beyond installation, the Argo CD Operator helps to automate the process
and remove the human as much as possible. This proposal falls into the automation domain
and enhance the Argo CD environment without human interaction.

### Goals

- Auto create/remove Argo CD cluster secrets using an external cluster registry.

### Non-Goals

- Support multiple external cluster registries for one Argo CD Operator instance.

## Proposal

Add a new optional field to
[ArgoCDSpec](https://github.com/argoproj-labs/argocd-operator/blob/master/api/v1alpha1/argocd_types.go)
to specify the name of the external cluster registry:

```
...
// ArgoCDSpec defines the desired state of ArgoCD
// +k8s:openapi-gen=true
type ArgoCDSpec struct {
...
	// ExternalClusterRegistry is used to specify an external cluster registry
  // for the operator to auto create and manage Argo CD cluster secrets. 
	ExternalClusterRegistry string `json:"externalClusterRegistry,omitempty"`
...
```

Modify the existing Argo CD controller
[util](https://github.com/argoproj-labs/argocd-operator/blob/master/controllers/argocd/util.go)
`reconcileResources` function to include:

```
if cr.Spec.ExternalClusterRegistry != nil {
  	log.Info("reconciling ExternalClusterRegistry")
		if err := r.reconcileExternalClusterRegistry(cr); err != nil {
			return err
		}
}
```

Create a new `go` file that handles the implementation of `reconcileExternalClusterRegistry`.
The function should first validate if the value of `ExternalClusterRegistry` is valid by
checking if the given value is an external cluster registry that Argo CD Operator supports.

If the value is valid, create a CronJob similar to
[argocdexport controller](https://github.com/argoproj-labs/argocd-operator/blob/master/controllers/argocdexport/job.go).
This CronJob will read the external cluster registry then create or remove
the associated Argo CD cluster secrets accordingly.

### Use cases

#### Use case 1:
As a user of an external cluster registry, I would like to the Argo CD Operator to help me auto create and manage
Argo CD cluster secrets based on the entries in the registry.

### Implementation Details/Notes/Constraints [optional]

Any feedbacks, ideas, and suggestions are most welcome.

### Detailed examples

Create a new Argo CD cluster in the `argocd` namespace using the provided following example:

```
apiVersion: argoproj.io/v1alpha1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: external-cluster-registry
spec:
  externalClusterRegistry: open-cluster-management-io
```

The operator will create the Argo CD cluster secrets based on the external cluster registry
implementation. See the following example:

```
apiVersion: v1
kind: Secret
metadata:
  name: cluster1-secret
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: cluster
type: Opaque
stringData:
  name: cluster1
  server: https://cluster1-control-plane:6443
```

### Security Considerations

* How does this proposal impact the security aspects of argocd-operator?

There might be some security risks by allowing third party implementations.
All the implementation code should live in the Argo CD Operator repo for code review and maintenance. 

* Are there any unresolved follow-ups that need to be done to make the enhancement more robust?

Consider the use of dynamic client for reading cluster registry instead of importing external
cluster registry APIs in the go module.

### Risks and Mitigations

This is an optional field and it should not impact existing Argo CD Operator users. 

### Upgrade / Downgrade Strategy

## Drawbacks

This proposal relies on different external cluster registry implementations. If there is a
standardize cluster registry API, then it's much easier to implement this proposal.

## Alternatives

Add clusters manually or using a script which are both not user friendly.
