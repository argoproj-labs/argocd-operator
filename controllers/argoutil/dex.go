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
	"fmt"
	"os"
)

const (
	DefaultDexStorageType         = "etcd"
	awkScriptEtcdStorageType      = "awk '/^storage:/ { print \"storage:\\n  type: etcd\\n  config:\\n    endpoints:\\n    - \\\"http://127.0.0.1:2379\\\"\\n    namespace: dex\"; skip=1; next } skip && /^[a-zA-Z0-9_-]+:/ { skip=0 } !skip' /tmp/base.yaml > /tmp/dex.yaml"
	customBootstrapScriptTemplate = `set -eo pipefail
trap 'kill -TERM $DEX_PID 2>/dev/null; exit 0' INT TERM

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
# run in a loop and restart the dex server process if there is a change in dex config.
while true; do
  /shared/argocd-dex gendexcfg ${EXTRA_ARGS} -o /tmp/base.yaml
  %s
  echo "starting dex server"
  dex serve /tmp/dex.yaml &
  DEX_PID=$!

  # continuously poll for changes to dex configuration in argocd-cm configmap
  # if a change is detected, send SIGTERM signal for dex server process for it to restart.
  while true; do
    sleep 15
    # check if the dex server process is running
    if ! kill -0 $DEX_PID 2>/dev/null; then
      echo "Dex process (PID $DEX_PID) exited unexpectedly. Restarting..."
      wait $DEX_PID 2>/dev/null || true
      break
    fi
    /shared/argocd-dex gendexcfg ${EXTRA_ARGS} -o /tmp/check_base.yaml 2>/dev/null || continue
    if [ "$(sha256sum < /tmp/base.yaml)" != "$(sha256sum < /tmp/check_base.yaml)" ]; then
      echo "Configuration change detected in argocd-cm/argocd-secret. Restarting Dex process..."
      kill -TERM $DEX_PID
      wait $DEX_PID 2>/dev/null || true
      break
    fi
  done
done`
)

// IsDexEtcdStorageEnabled returns a feature flag which determines if the dex storage config
// need to be overridden through env overrides. Returns false if explicitly disabled, true otherwise.
func IsDexEtcdStorageEnabled() bool {
	return getDexStorageType() == "etcd"
}

// DexServerCustomStartupScript returns the script that is required for generating dex config from `argocd-cm` config map,
// updating the dex storage to kubernetes and generate TLS certs and start the dex server.
func DexServerCustomStartupScript() []string {
	return []string{
		fmt.Sprintf(customBootstrapScriptTemplate, awkScriptEtcdStorageType),
	}
}

// getDexStorageType returns the storage type that needs to be used for dex.
func getDexStorageType() string {
	if env := os.Getenv("ARGOCD_DEX_STORAGE_TYPE"); env != "" {
		return env
	}
	return DefaultDexStorageType
}
