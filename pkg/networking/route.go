package networking

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	routev1 "github.com/openshift/api/route/v1"
)

var routeAPIFound = false

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

// SetRouteAPIFound sets the value of routeAPIFound to provided input
func SetRouteAPIFound(found bool) {
	routeAPIFound = found
}

// verifyRouteAPI will verify that the Route API is present.
func VerifyRouteAPI() error {
	found, err := argoutil.VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}
