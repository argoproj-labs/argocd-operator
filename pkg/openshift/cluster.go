package openshift

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	"gopkg.in/yaml.v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

var (
	versionAPIFound = false
	isOpenShiftEnv  = false
)

// IsOpenShiftEnv returns true if the present environment is an OpenShift cluster
func IsOpenShiftEnv() bool {
	isOpenShiftEnv = IsVersionAPIAvailable()
	return isOpenShiftEnv
}

// SetIsOpenShiftEnv sets the value of isOpenShiftEnv to provided input
func SetIsOpenShiftEnv(val bool) {
	isOpenShiftEnv = val
}

// IsVersionAPIAvailable returns true if the OpenShift cluster version api is present
func IsVersionAPIAvailable() bool {
	return versionAPIFound
}

// SetVersionAPIFound sets the value of versionAPIFound to provided input
func SetVersionAPIFound(found bool) {
	versionAPIFound = found
}

// verifyVersionAPI will verify that the template API is present.
func VerifyVersionAPI() error {
	found, err := argoutil.VerifyAPI(configv1.GroupName, configv1.GroupVersion.Version)
	if err != nil {
		return err
	}
	versionAPIFound = found
	return nil
}

// GetClusterVersion returns the OpenShift Cluster version in which the operator is installed
func GetClusterVersion(client client.Client) (string, error) {
	if !IsVersionAPIAvailable() {
		return "", nil
	}
	clusterVersion := &configv1.ClusterVersion{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: "version"}, clusterVersion)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return clusterVersion.Status.Desired.Version, nil
}

func GetOpenShiftAPIURL() (string, error) {
	k8s, err := argoutil.GetK8sClient()
	if err != nil {
		return "", fmt.Errorf("GetOpenShiftAPIURL: failed to initialize k8s client: %w", err)
	}

	cm, err := k8s.CoreV1().ConfigMaps("openshift-console").Get(context.TODO(), "console-config", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("GetOpenShiftAPIURL: failed to retrieve configmap console-config: %w", err)
	}

	var cf string
	if v, ok := cm.Data["console-config.yaml"]; ok {
		cf = v
	}

	data := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(cf), data)
	if err != nil {
		return "", fmt.Errorf("GetOpenShiftAPIURL: failed to unmarshal configmap console-config: %w", err)
	}

	var apiURL interface{}
	var out string
	if c, ok := data["clusterInfo"]; ok {
		ci, _ := c.(map[interface{}]interface{})

		apiURL = ci["masterPublicURL"]
		out = fmt.Sprintf("%v", apiURL)
	}

	return out, nil
}

func IsProxyCluster() (bool, error) {
	configClient, err := argoutil.GetConfigClient()
	if err != nil {
		return false, fmt.Errorf("IsProxyCluster: could not get config client: %w", err)
	}

	proxy, err := configClient.Proxies().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("IsProxyCluster: could not get proxy: %w", err)
	}

	if proxy.Spec.HTTPSProxy != "" {
		return true, nil
	}

	return false, nil
}
