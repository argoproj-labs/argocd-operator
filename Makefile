# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.13.0

# Try to detect Docker or Podman
CONTAINER_RUNTIME := $(shell command -v docker 2> /dev/null || command -v podman 2> /dev/null)

# If neither Docker nor Podman is found, print an error message and exit
ifeq ($(CONTAINER_RUNTIME),)
$(warning "No container runtime (Docker or Podman) found in PATH. Please install one of them.")
endif

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Set the Operator SDK version to use.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.35.0


# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# /argocd-operator-bundle:$VERSION and /argocd-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/argoprojlabs/argocd-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):v$(VERSION)

LD_FLAGS = "-X github.com/argoproj-labs/argocd-operator/version.Version=$(VERSION)"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

test: manifests generate fmt vet envtest ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -ldflags=$(LD_FLAGS) -o bin/manager cmd/main.go

run: manifests generate fmt vet ## Run a controller from your host.
	REDIS_CONFIG_PATH="build/redis" go run -ldflags=$(LD_FLAGS) ./cmd/main.go

docker-build: test ## Build docker image with the manager.
	$(CONTAINER_RUNTIME) build --build-arg LD_FLAGS=$(LD_FLAGS) -t ${IMG} .

docker-push: ## Push docker image with the manager.
	$(CONTAINER_RUNTIME) push ${IMG}

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)


.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
endif


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	## TODO: Remove sed usage after all v1alpha1 references are updated to v1beta1 in codebase.
	## For local testing, conversion webhook defined in crd makes call to webhook for each v1alpha1 reference
	## causing failures as we don't set up the webhook for local testing.
	$(KUSTOMIZE) build config/crd | sed '/conversion:/,/- v1beta1/d' |kubectl apply --server-side=true -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply --server-side=true -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ E2E

e2e: ## Run operator e2e tests
	kubectl kuttl test ./tests/k8s --config ./tests/kuttl-tests.yaml

all: test install run e2e ## UnitTest, Run the operator locally and execute e2e tests.

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.5.2)

ENVTEST = $(shell pwd)/bin/setup-envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)
	$(ENVTEST) use 1.26

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

.PHONY: bundle
bundle: operator-sdk manifests kustomize ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
	sed -i 's/control-plane: argocd-operator/control-plane: controller-manager/g' bundle/manifests/argocd-operator-webhook-service_v1_service.yaml bundle/manifests/argocd-operator-controller-manager-metrics-service_v1_service.yaml bundle/manifests/argocd-operator.clusterserviceversion.yaml
	rm -fr deploy/olm-catalog/argocd-operator/$(VERSION)
	mkdir -p deploy/olm-catalog/argocd-operator/$(VERSION)
	cp -r bundle/manifests/* deploy/olm-catalog/argocd-operator/$(VERSION)/
	mv deploy/olm-catalog/argocd-operator/$(VERSION)/argocd-operator.clusterserviceversion.yaml deploy/olm-catalog/argocd-operator/$(VERSION)/argocd-operator.v$(VERSION).clusterserviceversion.yaml

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	$(CONTAINER_RUNTIME) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

# UTIL_IMG defines the image:tag used for the utility image (for backup).
# You can use it as an arg. (E.g make bundle-build UTIL_IMG=<some-registry>/<project-name-bundle>:<tag>)
UTIL_IMG ?= $(IMAGE_TAG_BASE)-util:v$(VERSION)

.PHONY: util-build
util-build: ## Build the util container image (for backup)
	$(CONTAINER_RUNTIME) build --no-cache -t $(UTIL_IMG) build/util

.PHONY: util-push
util-push: ## Push the util container image
	$(MAKE) docker-push IMG=$(UTIL_IMG)

# REGISTRY_IMG defines the image:tag used for the (legacy) registry container image.
# You can use it as an arg. (E.g make bundle-build UTIL_IMG=<some-registry>/<project-name-bundle>:<tag>)
REGISTRY_IMG ?= $(IMAGE_TAG_BASE)-registry:v$(VERSION)

.PHONY: registry-build
registry-build: ## Build the registry container image
	rm -fr build/_output
	mkdir -p build/_output/registry
	cp -r deploy/registry/* build/_output/registry/
	mkdir -p build/_output/registry/manifests
	cp -r deploy/olm-catalog/argocd-operator build/_output/registry/manifests/
	$(CONTAINER_RUNTIME) build -t $(REGISTRY_IMG) build/_output/registry

.PHONY: registry-push
registry-push: ## Push the util container image
	$(MAKE) docker-push IMG=$(REGISTRY_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool $(CONTAINER_RUNTIME) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)
