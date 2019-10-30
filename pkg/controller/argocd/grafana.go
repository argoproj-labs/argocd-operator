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
	"path/filepath"
	"strings"
	"text/template"
)

const (
	grafanaConfigDir = "/var/lib/grafana"
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

// loadGrafanaConfigs will scan the config directory and read any files ending with '.yaml'
func loadGrafanaConfigs() (map[string]string, error) {
	data := make(map[string]string)

	pattern := filepath.Join(grafanaConfigDir, "*.yaml")
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

	templateDir := filepath.Join(grafanaConfigDir, "templates")
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
