# Install

The Argo CD Operator was created with the intention of running through the [Operator Lifecycle Manager][olm_home], 
specifically on [OpenShift 4][openshift_home]. This is where the operator shines most, as it leverages the powerful 
features built into OpenShift 4.

That being said, the operator can be installed and provide the same functionality on any Kubernetes cluster. The 
following methods are provided for installing the operator.

## OpenShift 4

Starting with `v0.0.4` the operator is published along with the set of built-in Community Operators in the Operator 
Hub on OpenShift 4. See the [OpenShift Install Guide][install_openshift] for more information on installing on the
OpenShift platorm.

## Operator Lifecycle Manager

Using the Operator Lifecycle Manager to install and manage the Argo CD Operator is the preferred method. Have a look 
at the [OLM Install Guide][install_olm] for details on this approach. 

## Manual Installation

The operator can be installed manually if desired.

!!! info
    The manual installation method requires cluster credentials that provide the `cluster-admin` ClusterRole or 
    equivalent.

The [Manual Installation Guide][install_manual] provides the steps needed to manually install the operator on any 
Kubernetes cluster.

[install_manual]:./install/manual.md
[install_olm]:./install/olm.md
[install_openshift]:./install/openshift.md
[olm_home]:https://github.com/operator-framework/operator-lifecycle-manager
[openshift_home]:https://try.openshift.com
