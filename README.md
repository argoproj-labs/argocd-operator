# Argo CD Operator

A Kubernetes operator for managing Argo CD deployments.

## Oeverview

The Argo CD Operator manages the full lifecycle for Argo CD and it's components. The operator aims to provide the follwing.

* Easy configuration and installation of the Argo CD components with sane defaults to get up and running quickly.
* Provide seamless upgrades to the Argo CD components.
* Ablity to back up and restore an Argo CD deployment from a point in time.
* Expose and aggregate the metrics for Argo CD and the operator itself using Prometheus and Grafana.
* Autoscale the Argo CD components as necessary to handle increased load.

## Deployment and User Guides

See the deployment and user [guides][guide_docs] for different platforms.

## Contributing

See the [development documentation][dev_docs] for information on how to contribute!

## License

ArgoCD Operator is released under the Apache 2.0 license. See the [LICENSE][license_file] file for details.

[guide_docs]:./docs/guides/
[dev_docs]:./docs/development.md
[license_file]:./LICENSE
