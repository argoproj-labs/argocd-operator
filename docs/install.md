# Install

The Argo CD Operator was created with the intention of running through the [Operator Lifecycle Manager][olm_home], specifically on 
[OpenShift 4][openshift_home]. This is where the operator shines most, as it leverages the powerful features built into OpenShift 4.x.

That being said, the operator can be installed and provide the same functionality on any Kubernetes cluster. The 
following methods are provided for installing the operator.

## OLM Install

Using the Operator Lifecycle Manager to install and manage the Argo CD Operator is the preferred method. Have a look 
at the [OLM Install Guide][install_olm] for details on this approach. 

## Manual Install

The operator can be installed manually if desired. It is worth mentioning that using this method requires cluster 
credentials that provide the `cluster-admin` ClusterRole or equivalent.

The [Basic Install Guide][install_basic] provides the steps 
needed to install the operator on any Kubernetes cluster. OpenShift users should also see the [OpenShift Install Guide][install_openshift].

[install_basic]:./guides/install-basic.md
[install_olm]:./guides/install-olm.md
[install_openshift]:./guides/install-openshift.md
[olm_home]:https://github.com/operator-framework/operator-lifecycle-manager
[openshift_home]:https://try.openshift.com
