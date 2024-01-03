## OLM Installed Operator

This folder contains tests which need operator installed via OLM

#### Steps for local testing via OLM

##### Setup local k8s cluster

```bash
# create local image registry
k3d registry create registry.localhost --port 12345

# ensure that below "k3d-registry.localhost" entry is present in your /etc/hosts file for local registry domain resolution
$ cat /etc/hosts
......
......
......
127.0.0.1 k3d-registry.localhost

# create a new cluster that uses this registry
k3d cluster create test --registry-use k3d-registry.localhost:12345
```

##### Build and install the operator

```bash
# install olm on target cluster
operator-sdk olm install

# env setup
export REGISTRY="k3d-registry.localhost:12345"
export ORG="local"
export CONTROLLER_IMAGE="argocd-operator:test"
export BUNDLE_IMAGE="argocd-operator-bundle:test"
export VERSION="0.9.0"
export CATALOG_IMAGE=argocd-operator-catalog
export CATALOG_TAG=v0.9.0

# build and push operator image
make generate
make manifests
make docker-build IMG="$REGISTRY/$ORG/$CONTROLLER_IMAGE"
docker push "$REGISTRY/$ORG/$CONTROLLER_IMAGE"

# build and push bundle image
make bundle IMG="$REGISTRY/$ORG/$CONTROLLER_IMAGE"
make bundle-build BUNDLE_IMG="$REGISTRY/$ORG/$BUNDLE_IMAGE"
docker push "$REGISTRY/$ORG/$BUNDLE_IMAGE"

# validate bundle
operator-sdk bundle validate "$REGISTRY/$ORG/$BUNDLE_IMAGE" 

# skip catalog and install operator on target cluster via operator-sdk
#operator-sdk run bundle "$REGISTRY/$ORG/$BUNDLE_IMAGE" -n operators

# build and push catalog image
make catalog-build CATALOG_IMG="$REGISTRY/$ORG/$CATALOG_IMAGE:$CATALOG_TAG" BUNDLE_IMGS="$REGISTRY/$ORG/$BUNDLE_IMAGE"
docker push "$REGISTRY/$ORG/$CATALOG_IMAGE:$CATALOG_TAG"

# install catalog
sed "s/image:.*/image: $REGISTRY\/$ORG\/$CATALOG_IMAGE:$CATALOG_TAG/" deploy/catalog_source.yaml | kubectl apply -n operators -f -

# create subscription
sed "s/sourceNamespace:.*/sourceNamespace: operators/" deploy/subscription.yaml | kubectl apply -n operators -f -
```

##### Run tests

```bash
# wait till argocd-operator-controller-manager pod is in running state
kubectl -n operators get all

kubectl kuttl test ./tests/olm --config ./tests/kuttl-tests.yaml
```

##### Cleanup

```bash
operator-sdk cleanup argocd-operator -n operators
kubectl delete clusterserviceversion/argocd-operator.v0.9.0 -n operators
make uninstall

k3d cluster delete test
k3d registry delete registry.localhost
```