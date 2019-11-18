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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
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
	replicas := ArgoCDDefaultGrafanaReplicas
	if cr.Spec.Prometheus.Size > replicas {
		replicas = cr.Spec.Grafana.Size
	}
	return &replicas
}

// getGrafanaConfigPath will return the path for the Grafana configuration templates
func getGrafanaConfigPath() string {
	path := os.Getenv("GRAFANA_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return ArgoCDDefaultGrafanaConfigPath
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

// loadGrafanaTemplates will scan the template directory and parse/execute any files ending with '.tmpl'
func loadGrafanaTemplates(c *GrafanaConfig) (map[string]string, error) {
	data := make(map[string]string)

	templateDir := filepath.Join(getGrafanaConfigPath(), "templates")
	entries, err := ioutil.ReadDir(templateDir)
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
