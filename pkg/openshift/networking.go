package openshift

import (
	"errors"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
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

// RouteRequest objects contain all the required information to produce a route object in return
type RouteRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       routev1.RouteSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newRoute returns a new Route instance for the given ArgoCD.
func newRoute(objectMeta metav1.ObjectMeta, spec routev1.RouteSpec) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func RequestRoute(request RouteRequest) (*routev1.Route, error) {
	var (
		mutationErr error
	)
	route := newRoute(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, route, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return route, fmt.Errorf("RequestRoute: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return route, nil
}

// CreateRoute creates the specified Route using the provided client.
func CreateRoute(route *routev1.Route, client cntrlClient.Client) error {
	return resource.CreateObject(route, client)
}

// UpdateRoute updates the specified Route using the provided client.
func UpdateRoute(route *routev1.Route, client cntrlClient.Client) error {
	return resource.UpdateObject(route, client)
}

// DeleteRoute deletes the Route with the given name and namespace using the provided client.
func DeleteRoute(name, namespace string, client cntrlClient.Client) error {
	route := &routev1.Route{}
	return resource.DeleteObject(name, namespace, route, client)
}

// GetRoute retrieves the Route with the given name and namespace using the provided client.
func GetRoute(name, namespace string, client cntrlClient.Client) (*routev1.Route, error) {
	route := &routev1.Route{}
	obj, err := resource.GetObject(name, namespace, route, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a routev1.Route
	route, ok := obj.(*routev1.Route)
	if !ok {
		return nil, errors.New("failed to assert the object as a routev1.Route")
	}
	return route, nil
}

// ListRoutes returns a list of Route objects in the specified namespace using the provided client and list options.
func ListRoutes(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*routev1.RouteList, error) {
	routeList := &routev1.RouteList{}
	obj, err := resource.ListObjects(namespace, routeList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a routev1.RouteList
	routeList, ok := obj.(*routev1.RouteList)
	if !ok {
		return nil, errors.New("failed to assert the object as a routev1.RouteList")
	}
	return routeList, nil
}
