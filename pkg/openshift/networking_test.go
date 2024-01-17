package openshift

import (
	"context"
	"sort"
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type routeOpt func(*routev1.Route)

func getTestRoute(opts ...routeOpt) *routev1.Route {
	desiredRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.AppK8sKeyName:      testInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
				common.AppK8sKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      testInstance,
				common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
			},
		},
		Spec: routev1.RouteSpec{
			TLS: &routev1.TLSConfig{
				Termination: "reencrypt",
			},
			To: routev1.RouteTargetReference{
				Name: testApplicationName,
			},
		},
	}

	for _, opt := range opts {
		opt(desiredRoute)
	}
	return desiredRoute
}

func TestRequestRoute(t *testing.T) {

	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	tests := []struct {
		name         string
		routeReq     RouteRequest
		desiredRoute *routev1.Route
		mutation     bool
		wantErr      bool
	}{
		{
			name: "request route, no mutation",
			routeReq: RouteRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: "reencrypt",
					},
					To: routev1.RouteTargetReference{
						Name: testApplicationName,
					},
				},
			},
			mutation:     false,
			desiredRoute: getTestRoute(func(r *routev1.Route) {}),
			wantErr:      false,
		},
		{
			name: "request route, no mutation, custom name, labels, annotations",
			routeReq: RouteRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
						testKey:                   testVal,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
						testKey:                           testVal,
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: "reencrypt",
					},
					To: routev1.RouteTargetReference{
						Name: testApplicationName,
					},
				},
			},
			mutation: false,
			desiredRoute: getTestRoute(func(r *routev1.Route) {
				r.Name = testName
				r.Labels = util.MergeMaps(r.Labels, testKVP)
				r.Annotations = util.MergeMaps(r.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request route, successful mutation",
			routeReq: RouteRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testRouteNameMutated,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: "reencrypt",
					},
					To: routev1.RouteTargetReference{
						Name: testApplicationName,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:     true,
			desiredRoute: getTestRoute(func(r *routev1.Route) { r.Name = testRouteNameMutated }),
			wantErr:      false,
		},
		{
			name: "request route, failed mutation",
			routeReq: RouteRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: "reencrypt",
					},
					To: routev1.RouteTargetReference{
						Name: testApplicationName,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:     true,
			desiredRoute: getTestRoute(func(r *routev1.Route) {}),
			wantErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRoute, err := RequestRoute(test.routeReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredRoute, gotRoute)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateRoute(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	desiredRoute := getTestRoute(func(r *routev1.Route) {
		r.TypeMeta = metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: "route.openshift.io/v1",
		}
		r.Name = testName
		r.Namespace = testNamespace
	})
	err := CreateRoute(desiredRoute, testClient)
	assert.NoError(t, err)

	createdRoute := &routev1.Route{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdRoute)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoute, createdRoute)
}

func TestGetRoute(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestRoute(func(r *routev1.Route) {
		r.Name = testName
		r.Namespace = testNamespace
	})).Build()

	_, err := GetRoute(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetRoute(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListRoutes(t *testing.T) {
	route1 := getTestRoute(func(r *routev1.Route) {
		r.Name = "route-1"
		r.Namespace = testNamespace
		r.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	route2 := getTestRoute(func(r *routev1.Route) { r.Name = "route-2" })
	route3 := getTestRoute(func(r *routev1.Route) {
		r.Name = "route-3"
		r.Namespace = testNamespace
		r.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		route1, route2, route3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredRoutes := []string{"route-1", "route-3"}

	existingRouteList, err := ListRoutes(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingRoutes := []string{}
	for _, route := range existingRouteList.Items {
		existingRoutes = append(existingRoutes, route.Name)
	}
	sort.Strings(existingRoutes)

	assert.Equal(t, desiredRoutes, existingRoutes)
}

func TestUpdateRoute(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))

	// Create the initial Route
	initialRoute := getTestRoute(func(r *routev1.Route) {
		r.Name = testName
		r.Namespace = testNamespace
	})

	// Create the client with the initial Route
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialRoute).Build()

	// Fetch the Route from the client
	desiredRoute := &routev1.Route{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: testName, Namespace: testNamespace}, desiredRoute)
	assert.NoError(t, err)

	desiredRoute.Spec.Port = &routev1.RoutePort{
		// TargetPort: intstr.IntOrString{IntVal: int32(9001)},
		TargetPort: intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "https",
		},
	}

	err = UpdateRoute(desiredRoute, testClient)
	assert.NoError(t, err)

	existingRoute := &routev1.Route{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRoute)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoute.Spec.Port, existingRoute.Spec.Port)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingRoute = getTestRoute(func(r *routev1.Route) {
		r.Name = testName
		r.Labels = nil
	})
	err = UpdateRoute(existingRoute, testClient)
	assert.Error(t, err)
}

func TestDeleteRoute(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))

	testRoute := getTestRoute(func(r *routev1.Route) {
		r.Name = testName
		r.Namespace = testNamespace
	})

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(testRoute).Build()

	err := DeleteRoute(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingRoute := &routev1.Route{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRoute)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
