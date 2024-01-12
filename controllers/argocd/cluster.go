package argocd

import (
	"os"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// VerifyClusterAPIs will verify the availability of extra features on the cluster, such as Prometheus and OpenShift Routes.
func VerifyClusterAPIs() error {
	var inspectError error

	if err := monitoring.VerifyPrometheusAPI(); err != nil {
		inspectError = err
	}

	if err := openshift.VerifyRouteAPI(); err != nil {
		inspectError = err
	}

	if err := openshift.VerifyTemplateAPI(); err != nil {
		inspectError = err
	}

	if err := openshift.VerifyVersionAPI(); err != nil {
		inspectError = err
	}

	return inspectError
}

// GetClusterConfigNamespaces returns the list of namespaces allowed to host cluster scoped instances
func GetClusterConfigNamespaces() []string {
	nsList := os.Getenv(common.ArgoCDClusterConfigNamespacesEnvVar)
	return util.SplitList(nsList)
}

// IsClusterConfigNs checks if the given namespace is allowed to host a cluster scoped instance
func IsClusterConfigNs(current string) bool {
	clusterConfigNamespaces := GetClusterConfigNamespaces()
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
