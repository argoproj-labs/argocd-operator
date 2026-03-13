# Argo CD Operator Release Process

!!! note 

    Currently argocd-operator can only support releasing versions which are the next highest release. Z-stream releases for minor versions which are not the highest minor version are not possible at this time. This functionality will be possible once argocd-operator switches from a single subscription channel (currently "alpha") model to release-based subscription channels, as [this GitHub issue](https://github.com/argoproj-labs/argocd-operator/issues/1436) explains.

## Prerequisites

Before beginning, make sure you have push access to the following repositories in quay.io:

  * [https://quay.io/argoprojlabs/argocd-operator-util](https://quay.io/argoprojlabs/argocd-operator-util)
  * [http://quay.io/argoprojlabs/argocd-operator](http://quay.io/argoprojlabs/argocd-operator)
  * [http://quay.io/argoprojlabs/argocd-operator-registry](http://quay.io/argoprojlabs/argocd-operator-registry) 

Lastly, make sure you are listed as a maintainer for argocd-operator in order to tag and publish releases. 

## `argocd-operator` changes

* `VERSION` in Makefile defines the project version for the bundle. You will need to update this value when you want to upgrade the version of your project.
  
```txt
  VERSION ?= 0.2.0
```

* Ensure that the `replaces` field in `config/manifests/bases/argocd-operator.clusterserviceversion.yaml` is set to the version you are planning to release.

* Ensure that the `currentCSV` field in `deploy/olm-catalog/argocd-operator/argocd-operator.package.yaml` is set to the version you are planning to release.

* Build the `argocd-operator-util` image.
  
```txt
  make util-build 
```

* Push the `argocd-operator-util` image to quay.io
  
```txt
  make util-push
```

* Copy the SHA digest of utility container image from the above command. Set this value to `ArgoCDDefaultExportJobVersion` in `common/defaults.go`.
     
* Build the operator container image. (Below command assumes the release version as `v0.2.0`; please change the command accordingly.)
  
```txt
  make docker-build IMG=quay.io/argoprojlabs/argocd-operator:v0.2.0-rc1
```

* Push the operator container image. (Below command assumes the release version as `v0.2.0`; please change the command accordingly.)
  
```txt
  make docker-push IMG=quay.io/argoprojlabs/argocd-operator:v0.2.0-rc1
```

* Create the bundle artifacts using the SHA of the operator container image.
  
```txt
  make bundle IMG=quay.io/argoprojlabs/argocd-operator@sha256:d894c0f7510c8f41b48900b52eac94f623885fd409ebf2660793cd921b137bde
```

* The step above will create some changes to the control-plane code that must be reverted:
    * In `bundle/manifests/argocd-operator.clusterserviceversion.yaml` and in `deploy/olm-catalog/argocd-operator/[your-version]/argocd-operator.[your-version].clusterserviceversion.yaml`, under the `spec.install.spec.deployments` add the control-plane label and change the deployment `spec.selector.matchLabels.control-plane` from `argocd-operator` to `controller-manager`, like so: 
```yaml
deployments:
  - label:
      control-plane: controller-manager
    name: argocd-operator-controller-manager
    spec:
      replicas: 1
      selector:
        matchLabels:
          control-plane: controller-manager
      strategy: {}
      template:
        metadata:
          labels:
            control-plane: controller-manager
```
  * In `bundle/manifests/argocd-operator-controller-manager-metrics-service_v1_service.yaml` and `deploy/olm-catalog/argocd-operator/[your-version]/argocd-operator-controller-manager-metrics-service_v1_service.yaml`, update the control-plane labels from `argocd-operator ` to `controller-manager`, like so: 

```yaml
metadata:
  creationTimestamp: null
  labels:
    control-plane: controller-manager
  name: argocd-operator-controller-manager-metrics-service
spec:
  ports:
  - name: https
    port: 8443
    targetPort: 8080
  selector:
    control-plane: controller-manager
```
  * In `bundle/manifests/argocd-operator-webhook-service_v1_service.yaml` and `deploy/olm-catalog/argocd-operator/[your-version]/argocd-operator-webhook-service_v1_service.yaml`, update the control-plane label from `argocd-operator` to `controller-manager`, like so: 
```yaml
selector:
  control-plane: controller-manager
```

* Create the registry image. (Below command assumes the release version as `v0.2.0`; please change the command accordingly.)
  
```txt
  make registry-build REGISTRY_IMG=quay.io/argoprojlabs/argocd-operator-registry:v0.2.0-rc1
```

* Push the registry image. (Below command assumes the release version as `v0.2.0`; please change the command accordingly.)
  
```txt
  make registry-push REGISTRY_IMG=quay.io/argoprojlabs/argocd-operator-registry:v0.2.0-rc1
```

* Update `deploy/catalog_source.yaml` with the SHA of the operator registry image.

* Once all testing has been done, from the quay.io user interface, add the actual release tags (e.g. 'v0.2.0') to the `argocd-operator` and `argocd-operator-registry` images.

* Commit and push the changes, then create a PR and get it merged.

* Go to the argocd-operator GitHub repo and [create (aka draft) a new release](https://github.com/argoproj-labs/argocd-operator/releases). Make sure to include release notes detailing what's changed and contributors. GitHub can help you generate new release notes (make sure to only include changes since the previous release, there is a dropdown option on the GitHub UI when drafting release notes to specify the previous tag).

----

## Steps to create a PR for Kubernetes OperatorHub Community Operators

* Fork and clone [kubernetes community operators](https://github.com/k8s-operatorhub/community-operators).

* Go to the `community-operators/operators/argocd-operator` folder.

* Create a new folder for the release with two child folders inside of it; one called `manifests` and one called `metadata`.

* In the `manifests` folder, copy and paste the files from the actual argocd-operator's `deploy/olm-catalog/argocd-operator/[release-version]` folder.

* Also in the `manifests` folder, edit the CSV file to add a `containerImage` tag to the metadata section. Copy the value from the `image` tag already found in the file.

* In the `metadata` folder, create a file called `annotations.yaml`. The content of this file can be copied from the previous argocd-operator release version in this repository. 

* Commit, sign and push the changes, then create a PR. The PR merge process should be automatic if all the checks pass; once the PR is merged then continue on to the next step. 

----

## Steps to create a PR for Red Hat Operators

* Fork and clone [redhat community operators](https://github.com/redhat-openshift-ecosystem/community-operators-prod).

* Go to the `community-operators-prod/operators/argocd-operator` folder.

* Create a new folder for the release with two child folders inside of it; one called `manifests` and one called `metadata`.

* In the `manifests` folder, copy and paste the files from the actual argocd-operator's `deploy/olm-catalog/argocd-operator/[release-version]` folder.

* Also in the `manifests` folder, edit the CSV file to add a `containerImage` tag to the metadata section. Copy the value from the `image` tag already found in the file.

* In the `metadata` folder, create a file called `annotations.yaml`. The content of this file can be copied from the previous argocd-operator release version in this repository. 

* Commit, sign and push the changes, then create a PR. The PR process should be automatic for this repository as well if all the checks pass. 

## Synchonizing changes back to master branch and setting up the next version

* In the `argocd-operator` repo, you have to synchronize the changes from the release branch back to the master branch. After doing this run `make bundle`. (Note: this will revert some of the changes you made earlier, but this is okay for the master branch. Without running `make bundle` the tests will not pass, and ignoring those and merging regardless will make all future PR's also fail.)

* Update the `VERSION` in the Makefile in the `argocd-operator` repo's master branch to the next version (e.g. from `0.2.0 to 0.3.0).

* In `config/manifests/bases/argocd-operator.clusterserviceversion.yaml`, update the `replaces:` field to be the current version (the one you just released), and the `version:` field to be the next version. 

* Run `make bundle` again to generate the initial bundle manifests for the next version. (You may need to also run `go mod vendor` and `go mod tidy`)

* Commit and push the changes, then create a PR to argocd-operator's master branch.