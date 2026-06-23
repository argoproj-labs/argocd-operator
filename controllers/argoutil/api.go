// Copyright 2019 ArgoCD Operator Developers
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

package argoutil

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// VerifyAPI will verify that the given group/version is present in the cluster.
func VerifyAPI(group string, version string) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return false, err
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return false, err
	}

	gv := schema.GroupVersion{
		Group:   group,
		Version: version,
	}

	if err = discovery.ServerSupportsVersion(k8s, gv); err != nil {
		// error, API not available, check if it is registered.
		log.Info(fmt.Sprintf("%s/%s API not available, checking if its registered", group, version))
		return IsAPIRegistered(group, version)
	}

	log.Info(fmt.Sprintf("%s/%s API verified", group, version))
	return true, nil
}

// IsAPIRegistered returns true if the API is registered irrespective of
// whether the API status is available or not.
func IsAPIRegistered(group string, version string) (bool, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return false, err
	}

	client, err := aggregator.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create a kube-aggregator client")
		return false, fmt.Errorf("unable to create a kube-aggregator client. error: %w", err)
	}

	_, err = client.ApiregistrationV1().APIServices().
		Get(context.TODO(), fmt.Sprintf("%s.%s", version, group), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("%s/%s API is not registered", group, version))
			return false, nil
		} else {
			message := fmt.Sprintf("%s/%s API registration check failed.", group, version)
			log.Error(err, message)
			return false, fmt.Errorf("%s error: %w", message, err)
		}
	}
	log.Info(fmt.Sprintf("%s/%s API is registered", group, version))
	return true, nil
}

var routeAPIFound = false

// IsRouteAPIAvailable returns true if the Route API is present.
func IsRouteAPIAvailable() bool {
	return routeAPIFound
}

// VerifyRouteAPI will verify that the Route API is present.
func VerifyRouteAPI() error {
	found, err := VerifyAPI(routev1.GroupName, routev1.GroupVersion.Version)
	if err != nil {
		return err
	}
	routeAPIFound = found
	return nil
}

func SetRouteAPIFound(found bool) {
	routeAPIFound = found
}
