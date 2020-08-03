// Copyright 2019 Argo CD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var routeAPIFound = false

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

// verifyRouteAPI will verify that the Prometheus API is present.
func verifyRouteAPI() error {
	found, err := VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}

// NewRoute returns a new Route instance for the given ArgoCD.
func NewRoute(meta metav1.ObjectMeta) *routev1.Route {
	return &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    common.LabelsForCluster(meta),
		},
	}
}

// NewRouteWithName returns a new Route with the given name and ArgoCD.
func NewRouteWithName(meta metav1.ObjectMeta, name string) *routev1.Route {
	route := NewRoute(meta)
	route.ObjectMeta.Name = name

	lbls := route.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	route.ObjectMeta.Labels = lbls

	return route
}

// NewRouteWithSuffix returns a new Route with the given name suffix for the ArgoCD.
func NewRouteWithSuffix(meta metav1.ObjectMeta, suffix string) *routev1.Route {
	return NewRouteWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix))
}
