// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitopspromoter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-operator/common"
)

func TestBuildPolicyRulesForControllerClusterRoles_UsesEnvVariable(t *testing.T) {
	t.Setenv(common.GitOpsPromoterControllerClusterRoleEnvName, "test-role")
	cr := makeTestArgoCD(withPromoterEnabled(true))

	policyRules := buildPolicyRulesForControllerClusterRoles(testCompName, cr)
	assert.Greater(t, len(policyRules), 0)
	assert.Equal(t, "test-role", policyRules[0].roleRefName)
}

func TestBuildPolicyRulesForAPIServerClusterRoles_UsesEnvVariable(t *testing.T) {
	t.Setenv(common.GitOpsPromoterAPIServerClusterRoleEnvName, "test-role")
	cr := makeTestArgoCD(withPromoterEnabled(true))

	policyRules := buildPolicyRulesForAPIServerClusterRoles(testCompName, cr)
	assert.Greater(t, len(policyRules), 0)
	assert.Equal(t, "test-role", policyRules[0].roleRefName)
}
