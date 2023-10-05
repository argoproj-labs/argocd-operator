package networking

import (
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
)

var routeAPIFound = false

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

func SetRouteAPIFound(routeFound bool) {
	routeAPIFound = routeFound
}

// verifyRouteAPI will verify that the Route API is present.
func VerifyRouteAPI() error {
	found, err := util.VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}
