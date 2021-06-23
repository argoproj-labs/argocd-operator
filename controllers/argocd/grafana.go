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
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/sethvargo/go-password/password"
)

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
