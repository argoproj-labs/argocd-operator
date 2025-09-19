#!/bin/bash

# Copyright 2025 ArgoCD Operator Developers
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# 	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eo pipefail

RECREATE="$1"

export ARGOCD_AGENT_PRINCIPAL_NAMESPACE=argocd # Should be same as ArgoCD instance namespace
KUBECTL=$(which kubectl)
OPENSSL=$(which openssl)

# Create a secret for the redis password
${KUBECTL} create secret generic argocd-redis -n argocd --from-literal=auth="$(${KUBECTL} get secret argocd-redis-initial-password -n argocd -o jsonpath='{.data.admin\.password}' | base64 -d)"

IPADDR=""
if test "$IPADDR" = ""; then
       IPADDR=$(kubectl -n ${ARGOCD_AGENT_PRINCIPAL_NAMESPACE} get svc argocd-agent-principal -o jsonpath='{.spec.clusterIP}')
fi

if ! command -v argocd-agentctl >/dev/null 2>&1; then
	echo "Please ensure argocd-agentctl binary is installed in your PATH" >&2
	exit 1
fi

echo "[*] Initializing PKI"
if ! argocd-agentctl pki inspect >/dev/null 2>&1; then
	argocd-agentctl pki init
	echo "  -> PKI initialized."
else
	echo "  -> Reusing existing agent PKI."
fi

echo "[*] Creating principal TLS configuration"
argocd-agentctl pki issue principal --upsert \
	--principal-namespace ${ARGOCD_AGENT_PRINCIPAL_NAMESPACE} \
	--ip "127.0.0.1,${IPADDR}"
echo "  -> Principal TLS config created."

echo "[*] Creating resouce proxy TLS configuration"
argocd-agentctl pki issue resource-proxy --upsert \
	--principal-namespace ${ARGOCD_AGENT_PRINCIPAL_NAMESPACE} \
	--ip "127.0.0.1,${IPADDR}"
echo "  -> Resource proxy TLS config created."

echo "[*] Creating JWT signing key and secret"
${OPENSSL} genpkey -algorithm RSA -out /tmp/jwt.key -pkeyopt rsa_keygen_bits:2048
${KUBECTL} create secret generic -n ${ARGOCD_AGENT_PRINCIPAL_NAMESPACE} argocd-agent-jwt --from-file=jwt.key=/tmp/jwt.key

AGENTS="agent-managed agent-autonomous"
for agent in ${AGENTS}; do
	echo "[*] Creating configuration for agent ${agent}"
	if test "$RECREATE" = "--recreate"; then
		echo "  -> Deleting existing cluster secret, if it exists"
		kubectl -n ${ARGOCD_AGENT_PRINCIPAL_NAMESPACE} delete --ignore-not-found secret cluster-${agent}
	fi
	if ! argocd-agentctl agent inspect ${agent} >/dev/null 2>&1; then
		echo "  -> Creating cluster secret for agent configuration"
		argocd-agentctl agent create ${agent} \
			--resource-proxy-username ${agent} \
			--resource-proxy-password ${agent} \
			--resource-proxy-server ${IPADDR}:443
	else
		echo "  -> Reusing existing cluster secret for agent configuration"
	fi
done
