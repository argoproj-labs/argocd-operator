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
)

const (
	DefaultDexStorageType         = "memory"
	defaultNoopScriptlet          = "cp /tmp/base.yaml /tmp/dex.yaml"
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
%s
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

// DexServerCustomStartupScript returns the script that is required for generating dex config from `argocd-cm` config map,
// updating the dex storage to kubernetes and generate TLS certs and start the dex server.
func DexServerCustomStartupScript() []string {
	storageType := getDexStorageType()
	return []string{
		fmt.Sprintf(customBootstrapScriptTemplate, getDexStorageHealthCheckScriptlet(storageType), getDexStorageModificationScriptlet(storageType)),
	}
}

// getDexStorageType returns the storage type that needs to be used for dex.
func getDexStorageType() string {
	return DefaultDexStorageType
}

// getDexStorageHealthCheck returns the script to perform health check to see if storage service is ready
// and accepting connections.
func getDexStorageHealthCheckScriptlet(storageType string) string {
	return ""
}

// getDexStorageModificationScript returns the script to perform modifications to the dex configuration file to
func getDexStorageModificationScriptlet(storageType string) string {
	return defaultNoopScriptlet
}
