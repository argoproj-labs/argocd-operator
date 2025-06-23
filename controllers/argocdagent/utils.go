// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"fmt"

	logr "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logr.Log.WithName("controller_agent")

func generateAgentResourceName(crName, compName string) string {
	return fmt.Sprintf("%s-agent-%s", crName, compName)
}

func buildLabelsForAgentPrincipal(crName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/component":  "principal",
		"app.kubernetes.io/name":       "argocd-agent-principal",
		"app.kubernetes.io/part-of":    "argocd-agent",
		"app.kubernetes.io/managed-by": crName,
	}
}
