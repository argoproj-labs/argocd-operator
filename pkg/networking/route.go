package networking

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
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

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newRoute returns a new Route instance for the given ArgoCD.
func newRoute(objectMeta metav1.ObjectMeta, spec routev1.RouteSpec) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func CreateRoute(route *routev1.Route, client cntrlClient.Client) error {
	return client.Create(context.TODO(), route)
}

// UpdateRoute updates the specified Route using the provided client.
func UpdateRoute(route *routev1.Route, client cntrlClient.Client) error {
	_, err := GetRoute(route.Name, route.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), route); err != nil {
		return err
	}
	return nil
}

func DeleteRoute(name, namespace string, client cntrlClient.Client) error {
	existingRoute, err := GetRoute(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingRoute); err != nil {
		return err
	}
	return nil
}

func GetRoute(name, namespace string, client cntrlClient.Client) (*routev1.Route, error) {
	existingRoute := &routev1.Route{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingRoute)
	if err != nil {
		return nil, err
	}
	return existingRoute, nil
}

func ListRoutes(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*routev1.RouteList, error) {
	existingRoutes := &routev1.RouteList{}
	err := client.List(context.TODO(), existingRoutes, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRoutes, nil
}

func RequestRoute(request RouteRequest) (*routev1.Route, error) {
	var (
		mutationErr error
	)
	route := newRoute(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, route, request.Client)
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
