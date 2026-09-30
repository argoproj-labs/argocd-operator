# 💤 The project is effectively dormant 💤

The development have moved to [redhat-developer/gitops-operator](https://github.com/redhat-developer/gitops-operator/). GitOps operator is available on OpenShift, and it will be made available for Kubernetes in the near future under the same license. Users are encouraged to use and contribute to the GitOps operator instead.

Over the past several years, it has been the contributions from Red Hat that has been pushing the operator(s) further. We did receive a number of inputs and PRs from the larger Argo CD community, but we conclude that we failed to create sufficient traction that would drive a measurable adoption and contribution to the upstream operator beyond Red Hat and our customers.

However, we will - partially for our own, partially for the interest of the community - publish an operator build made to run on Kubernetes. Existing deployments of argocd-operator will be able to migrate to gitops-operator (both use Apache-2.0 license), CRDs will be compatible initially, etc.

Thank you for your contributions and support, we hope to see you on the other side!
