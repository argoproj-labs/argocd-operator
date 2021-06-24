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

package argocd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/sethvargo/go-password/password"
)

// GrafanaConfig represents the Grafana configuration options.
type GrafanaConfig struct {
	// Security options
	Security GrafanaSecurityConfig
}

// GrafanaSecurityConfig represents the Grafana security options.
type GrafanaSecurityConfig struct {
	// AdminUser is the default admin user.
	AdminUser string

	// AdminPassword is the default admin password
	AdminPassword string

	// SecretKey is used for signing
	SecretKey string
}

// GetGrafanaHost will return the hostname value for Grafana.
func GetGrafanaHost(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("grafana", cr)
	if len(cr.Spec.Grafana.Host) > 0 {
		host = cr.Spec.Grafana.Host
	}
	return host
}

// GenerateGrafanaSecretKey will generate and return the secret key for Grafana.
func GenerateGrafanaSecretKey() ([]byte, error) {
	key, err := password.Generate(
		common.ArgoCDDefaultGrafanaSecretKeyLength,
		common.ArgoCDDefaultGrafanaSecretKeyNumDigits,
		common.ArgoCDDefaultGrafanaSecretKeyNumSymbols,
		false, false)

	return []byte(key), err
}

// LoadGrafanaConfigs will scan the config directory and read any files ending with '.yaml'
func LoadGrafanaConfigs() (map[string]string, error) {
	data := make(map[string]string)

	pattern := filepath.Join(GetGrafanaConfigPath(), "*.yaml")
	configs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, f := range configs {
		config, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(config)
	}

	return data, nil
}

// GetGrafanaConfigPath will return the path for the Grafana configuration templates
func GetGrafanaConfigPath() string {
	path := os.Getenv("GRAFANA_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultGrafanaConfigPath
}
