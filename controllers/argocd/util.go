// Copyright 2021 ArgoCD Operator Developers
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

package argocd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/google/martian/log"
	"github.com/sethvargo/go-password/password"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logr = logf.Log.WithName("controller_argocd")

// DexConnector represents an authentication connector for Dex.
type DexConnector struct {
	Config map[string]interface{} `yaml:"config,omitempty"`
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
}

// ArgocdInstanceSelector creates a label selector with "managed-by" requirement
func ArgocdInstanceSelector(name string) (labels.Selector, error) {
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(common.ArgoCDKeyManagedBy, selection.Equals, []string{name})
	if err != nil {
		return nil, fmt.Errorf("failed to create a requirement for %w", err)
	}
	return selector.Add(*requirement), nil
}

// FilterObjectsBySelector invokes list() on the client to return filtered objects
func FilterObjectsBySelector(c client.Client, objectList client.ObjectList, selector labels.Selector) error {
	return c.List(context.TODO(), objectList, client.MatchingLabelsSelector{Selector: selector})
}

// RemoveString removes a string from a slice
func RemoveString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// labelsForCluster returns the labels for all cluster resources.
func labelsForCluster(cr *argoprojv1a1.ArgoCD) map[string]string {
	labels := argoutil.DefaultLabels(cr.Name)
	for key, val := range cr.ObjectMeta.Labels {
		labels[key] = val
	}
	return labels
}

// AllowedNamespace returns true if current is found in namespaces
func AllowedNamespace(current string, namespaces string) bool {
	clusterConfigNamespaces := splitList(namespaces)
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

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

// NameWithSuffix will return a name based on the given ArgoCD. The given suffix is appended to the generated name.
// Example: Given an ArgoCD with the name "example-argocd", providing the suffix "foo" would result in the value of
// "example-argocd-foo" being returned.
func NameWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// GenerateArgoAdminPassword will generate and return the admin password for Argo CD.
func GenerateArgoAdminPassword() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultAdminPasswordLength,
		common.ArgoCDDefaultAdminPasswordNumDigits,
		common.ArgoCDDefaultAdminPasswordNumSymbols,
		false, false)

	return []byte(pass), err
}

// GenerateArgoServerSessionKey will generate and return the server signature key for session validation.
func GenerateArgoServerSessionKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultServerSessionKeyLength,
		common.ArgoCDDefaultServerSessionKeyNumDigits,
		common.ArgoCDDefaultServerSessionKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// GetDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func GetDexOAuthClientID(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName))
}

// GetRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisInitScript(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		logr.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// loadTemplateFile will parse a template with the given path and execute it with the given params.
func loadTemplateFile(path string, params map[string]string) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		logr.Error(err, "unable to parse template")
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		logr.Error(err, "unable to execute template")
		return "", err
	}
	return buf.String(), nil
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// GetRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisHAProxyConfig(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// GetRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisHAProxyScript(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// GetRedisConf will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisConf(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/redis.conf.tpl", getRedisConfigPath())
	conf, err := loadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis configuration")
		return ""
	}
	return conf
}

// GetRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisSentinelConf(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/sentinel.conf.tpl", getRedisConfigPath())
	conf, err := loadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis sentinel configuration")
		return ""
	}
	return conf
}
