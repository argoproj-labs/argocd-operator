# Argo CD Operator

A Kubernetes operator for managing Argo CD deployments.

## Overview

The Argo CD Operator manages the full lifecycle for [Argo CD][argocd_home] and it's components. The operator's goal is
to automate the tasks required when operating an Argo CD deployment.

Beyond installation, the operator helps to automate the process of upgrading, backing up and restoring as needed and
remove the human as much as possible. In addition, the operator aims to provide deep insights into the Argo CD
environment by configuring Prometheus and Grafana to aggregate, visualize and expose the metrics already exported by
Argo CD.

The operator aims to provide the following, and is a work in progress.

* Easy configuration and installation of the Argo CD components with sane defaults to get up and running quickly.
* Provide seamless upgrades to the Argo CD components.
* Ablity to back up and restore an Argo CD deployment from a point in time or recurring schedule.
* Aggregate and expose the metrics for Argo CD and the operator itself using Prometheus and Grafana.
* Autoscale the Argo CD components as necessary to handle variability in demand.

[argocd_home]:https://argoproj.github.io/projects/argo-cd
