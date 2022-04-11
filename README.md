# Argo CD Operator

[![Build Status](https://travis-ci.org/argoproj-labs/argocd-operator.svg?branch=master)](https://travis-ci.org/argoproj-labs/argocd-operator)
[![Go Report Card](https://goreportcard.com/badge/argoproj-labs/argocd-operator "Go Report Card")](https://goreportcard.com/report/argoproj-labs/argocd-operator)
[![Documentation Status](https://readthedocs.org/projects/argocd-operator/badge/?version=latest)](https://argocd-operator.readthedocs.io/en/latest/?badge=latest)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](code-of-conduct.md) 

A Kubernetes operator for managing Argo CD clusters.

## Documentation

See the [documentation][docs] for installation and usage of the operator.

## E2E testing

E2E tests are written using [KUTTL](https://kuttl.dev/docs/#install-kuttl-cli). Please Install [KUTTL](https://kuttl.dev/docs/#install-kuttl-cli) to run the tests.

Note that the e2e tests for Redis HA mode require a cluster with at least three worker nodes.  A local three-worker node
cluster can be created using [k3d](https://k3d.io/)

## License

The Argo CD Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[docs]:https://argocd-operator.readthedocs.io
[license_file]:./LICENSE
