package argocd

import (
	"os"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
)

// InspectCluster will verify the availability of extra features on the cluster, such as Prometheus and OpenShift Routes.
func InspectCluster() {
	// if err := monitoring.VerifyPrometheusAPI(); err != nil {
	// 	// TO DO: log error verifying prometheus API (warn)
	// }

	// if err := networking.VerifyRouteAPI(); err != nil {
	// 	// TO DO: log error verifying route API (warn)
	// }

	// if err := workloads.VerifyTemplateAPI(); err != nil {
	// 	// TO DO: log error verifying template API (warn)
	// }

	if err := cluster.VerifyVersionAPI(); err != nil {
		// TO DO: log error verifying version API (warn)
	}
}

func GetClusterConfigNamespaces() string {
	return os.Getenv(common.ArgoCDClusterConfigNamespacesEnvVar)
}

func IsClusterConfigNs(current string) bool {
	clusterConfigNamespaces := argoutil.SplitList(GetClusterConfigNamespaces())
	if len(clusterConfigNamespaces) > 0 {
		if clusterConfigNamespaces[0] == "*" {
			return true
		}

		for _, n := range clusterConfigNamespaces {
			if n == current {
				return true
			}
		}
	}
	return false
}
