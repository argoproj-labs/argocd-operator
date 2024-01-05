package argoutil

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// VerifyAPI will verify that the given group/version is present in the cluster.
func VerifyAPI(group string, version string) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return false, fmt.Errorf("VerifyAPI: unable to get k8s config: %w", err)
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return false, fmt.Errorf("VerifyAPI: unable to create k8s client: %w", err)
	}

	gv := schema.GroupVersion{
		Group:   group,
		Version: version,
	}

	if err = discovery.ServerSupportsVersion(k8s, gv); err != nil {
		// error, API not available
		return false, nil
	}

	return true, nil
}
