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

package argoutil

import (
	"os"
)

// IsDexKubernetesStorageEnabled returns a feature flag which determines if the dex storage config
// need to be overridden through env overrides. Returns false if explicitly disabled, true otherwise.
func IsDexKubernetesStorageEnabled() bool {
	return os.Getenv("ARGOCD_DEX_KUBERNETES_STORAGE_ENABLED") != "false"
}

// DexServerCustomStartupScript returns the script that is required for generating dex config from `argocd-cm` config map,
// updating the dex storage to kubernetes and generate TLS certs and start the dex server.
func DexServerCustomStartupScript() []string {
	return []string{
		`set -e
EXTRA_ARGS=""
if [ -s /tls/tls.crt ] && [ -s /tls/tls.key ]; then
  cp /tls/tls.crt /tmp/tls.crt
  cp /tls/tls.key /tmp/tls.key
elif command -v openssl >/dev/null 2>&1; then
      openssl req -x509 -newkey rsa:2048 -nodes \
      -keyout /tmp/tls.key -out /tmp/tls.crt -days 3650 \
      -subj "/CN=dexserver" -addext "subjectAltName=DNS:localhost,DNS:dexserver"
else
    EXTRA_ARGS="--disable-tls"
fi
/shared/argocd-dex gendexcfg ${EXTRA_ARGS} -o /tmp/base.yaml
awk '/^storage:/ { print "storage:\n  type: kubernetes\n  config:\n    inCluster: true"; skip=1; next } skip && /^[a-zA-Z0-9_-]+:/ { skip=0 } !skip' /tmp/base.yaml > /tmp/dex.yaml
exec dex serve /tmp/dex.yaml`,
	}
}
