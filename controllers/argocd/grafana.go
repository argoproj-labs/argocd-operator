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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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

// generateGrafanaSecretKey will generate and return the secret key for Grafana.
func generateGrafanaSecretKey() ([]byte, error) {
	key, err := password.Generate(
		common.ArgoCDDefaultGrafanaSecretKeyLength,
		common.ArgoCDDefaultGrafanaSecretKeyNumDigits,
		common.ArgoCDDefaultGrafanaSecretKeyNumSymbols,
		false, false)

	return []byte(key), err
}

// getGrafanaHost will return the hostname value for Grafana.
func getGrafanaHost(cr *argoproj.ArgoCD) string {
	host := nameWithSuffix("grafana", cr)
	if len(cr.Spec.Grafana.Host) > 0 {
		host = cr.Spec.Grafana.Host
	}
	return host
}

// getGrafanaReplicas will return the size value for the Grafana replica count.
func getGrafanaReplicas(cr *argoproj.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultGrafanaReplicas
	if cr.Spec.Grafana.Size != nil {
		if *cr.Spec.Grafana.Size >= 0 && *cr.Spec.Grafana.Size != replicas {
			replicas = *cr.Spec.Grafana.Size
		}
	}
	return &replicas
}

// getGrafanaConfigPath will return the path for the Grafana configuration templates
func getGrafanaConfigPath() string {
	path := os.Getenv("GRAFANA_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultGrafanaConfigPath
}

// hasGrafanaSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasGrafanaSpecChanged(actual *appsv1.Deployment, desired *argoproj.ArgoCD) bool {
	// Replica count
	if desired.Spec.Grafana.Size != nil { // Replica count specified in desired state
		if *desired.Spec.Grafana.Size >= 0 && *actual.Spec.Replicas != *desired.Spec.Grafana.Size {
			return true
		}
	} else { // Replica count NOT specified in desired state
		if *actual.Spec.Replicas != common.ArgoCDDefaultGrafanaReplicas {
			return true
		}
	}
	return false
}

// loadGrafanaConfigs will scan the config directory and read any files ending with '.yaml'
func loadGrafanaConfigs() (map[string]string, error) {
	data := make(map[string]string)

	pattern := filepath.Join(getGrafanaConfigPath(), "*.yaml")
	configs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, f := range configs {
		config, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(config)
	}

	return data, nil
}

// loadGrafanaTemplates will scan the template directory and parse/execute any files ending with '.tmpl'
func loadGrafanaTemplates(c *GrafanaConfig) (map[string]string, error) {
	data := make(map[string]string)

	templateDir := filepath.Join(getGrafanaConfigPath(), "templates")
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue // Ignore directories and anything that doesn't end with '.tmpl'
		}

		filename := entry.Name()
		path := filepath.Join(templateDir, filename)
		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		err = tmpl.Execute(buf, c)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(filename, ".tmpl")
		if len(parts) <= 1 {
			return nil, fmt.Errorf("invalid template name: %s", filename)
		}

		key := parts[0]
		data[key] = buf.String()
	}

	return data, nil
}
