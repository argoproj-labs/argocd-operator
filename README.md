# Argo CD Operator
[![Build Status](https://travis-ci.org/jmckind/argocd-operator.svg?branch=master)](https://travis-ci.org/jmckind/argocd-operator)
[![Go Report Card](https://goreportcard.com/badge/jmckind/argocd-operator "Go Report Card")](https://goreportcard.com/report/jmckind/argocd-operator)

A Kubernetes operator for managing Argo CD deployments.

## Overview

The Argo CD Operator is intended to manage the full lifecycle for [Argo CD][argocd_home] and it's components. The 
operator's goal is to automate the tasks required when operating Argo CD. Beyond installation, the operator attempts to  
automate the process of upgrading, backing up and restoring as needed and remove the human as much as possible.

In addition, the operator aims to provide deep insights into the Argo CD environment by configuring Prometheus and 
Grafana to expose, aggregate and visualize the metrics already exported by Argo CD. 

The operator aims to provide the following and is a work in progress.

* Easy configuration and installation of the Argo CD components with sane defaults to get up and running quickly.
* Provide seamless upgrades to the Argo CD components.
* Ablity to back up and restore an Argo CD deployment from a point in time.
* Expose and aggregate the metrics for Argo CD and the operator itself using Prometheus and Grafana.
* Autoscale the Argo CD components as necessary to handle increased load.

## Install

The Argo CD Operator can be installed in a variety of ways. 
Have a look at the [installation][docs_install] documentation for the supported methods.

## Usage 

Check out the [usage][docs_usage] documentation for examples of configuring and using the Argo CD Operator. 

## Contributing

Anyone interested in contributing to the Argo CD operator is welcomes and 
should start by reviewing the [development][docs_dev] documentation.

## License

ArgoCD Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[argocd_home]:https://argoproj.github.io/projects/argo-cd
[docs_dev]:./docs/development.md
[docs_install]:./docs/install.md
[docs_usage]:./docs/usage.md
[license_file]:./LICENSE
