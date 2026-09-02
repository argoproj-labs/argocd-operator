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
	"bytes"
	"context"
	"fmt"
	"os"
	"text/template"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigyaml "sigs.k8s.io/yaml"
)

const (
	dexCRDTemplate = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: {{ .Plural }}.dex.coreos.com
spec:
  group: dex.coreos.com
  names:
    kind: {{ .Kind }}
    listKind: {{ .Kind }}List
    plural: {{ .Plural }}
  scope: Namespaced
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
`
)

type CRDData struct {
	Kind   string
	Plural string
}

var DexCRDs = []CRDData{
	{Kind: "AuthCode", Plural: "authcodes"},
	{Kind: "AuthRequest", Plural: "authrequests"},
	{Kind: "Connector", Plural: "connectors"},
	{Kind: "DeviceRequest", Plural: "devicerequests"},
	{Kind: "DeviceToken", Plural: "devicetokens"},
	{Kind: "OAuth2Client", Plural: "oauth2clients"},
	{Kind: "OfflineSessions", Plural: "offlinesessionses"},
	{Kind: "Password", Plural: "passwords"},
	{Kind: "RefreshToken", Plural: "refreshtokens"},
	{Kind: "SigningKey", Plural: "signingkeies"},
}

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
trap 'kill -TERM $DEX_PID 2>/dev/null; exit 0' INT TERM
while true; do
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
  
  echo "starting dex server"
  dex serve /tmp/dex.yaml &
  DEX_PID=$!

  # continuously poll for changes to dex configuration in argocd-cm configmap
  # if a change is detected, send SIGTERM signal for dex server process for it to restart.
  while true; do
    sleep 15
    /shared/argocd-dex gendexcfg ${EXTRA_ARGS} -o /tmp/check_base.yaml 2>/dev/null || continue
    if [ "$(sha256sum < /tmp/base.yaml)" != "$(sha256sum < /tmp/check_base.yaml)" ]; then
      echo "Configuration change detected in argocd-cm/argocd-secret. Restarting Dex process..."
      kill -TERM $DEX_PID
      wait $DEX_PID 2>/dev/null || true
      break
    fi
  done
done`,
	}
}

// EnsureDexCRDs ensures that the dex CRDS are available. If not available, it creates it, and if already available,
// it will be a no-op.
func EnsureDexCRDs(ctx context.Context, c client.Client) error {
	tmpl, err := template.New("crd").Parse(dexCRDTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse CRD template: %w", err)
	}

	for _, item := range DexCRDs {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, item); err != nil {
			return fmt.Errorf("template execution failed for %s: %w", item.Kind, err)
		}

		obj := &unstructured.Unstructured{}
		if err := sigyaml.Unmarshal(buf.Bytes(), &obj.Object); err != nil {
			return fmt.Errorf("failed to unmarshal YAML into unstructured for %s: %w", item.Kind, err)
		}

		existing := &unstructured.Unstructured{}
		existing.SetGroupVersionKind(obj.GroupVersionKind())
		err = c.Get(ctx, client.ObjectKey{Name: obj.GetName()}, existing)
		if errors.IsNotFound(err) {
			// Create CRD
			if err := c.Create(ctx, obj); err != nil {
				return fmt.Errorf("failed to create CRD %s: %w", obj.GetName(), err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to fetch CRD %s: %w", obj.GetName(), err)
		}
	}
	return nil
}

// CanCreateDexCRDs checks if the operator's ServiceAccount has RBAC permission to create CRDs.
func CanCreateDexCRDs(ctx context.Context, c client.Client) (bool, error) {
	ssar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    "apiextensions.k8s.io",
				Resource: "customresourcedefinitions",
				Verb:     "create",
			},
		},
	}

	if err := c.Create(ctx, ssar); err != nil {
		return false, fmt.Errorf("failed to evaluate SelfSubjectAccessReview: %w", err)
	}

	return ssar.Status.Allowed, nil
}
