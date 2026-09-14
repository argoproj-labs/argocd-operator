package argocd

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	defaultDexStorageType         = "memory"
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

// getDexContainerImage will return the container image for the Dex server.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.sso.dex field has an image and version to use for
// generating an image reference.
// 2. from the Environment, this looks for the `ARGOCD_DEX_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDDefaultDexVersion and
// common.ArgoCDDefaultDexImage.
func getDexContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false

	img := ""
	tag := ""

	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Image != "" {
		img = cr.Spec.SSO.Dex.Image
	}

	if img == "" {
		img = common.ArgoCDDefaultDexImage
		defaultImg = true
	}

	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Version != "" {
		tag = cr.Spec.SSO.Dex.Version
	}

	if tag == "" {
		tag = common.ArgoCDDefaultDexVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDDexImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ReconcileArgoCD) getDexOAuthRedirectURI(cr *argoproj.ArgoCD) (string, error) {
	uri, err := r.getArgoServerURI(cr)
	if err != nil {
		return "", err
	}
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath, nil
}

// getDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func getDexOAuthClientID(cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, getServiceAccountName(cr.Name, common.ArgoCDDefaultDexServiceAccountName))
}

// getDexResources will return the ResourceRequirements for the Dex container.
func getDexResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {

	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.Resources != nil {
		resources = *cr.Spec.SSO.Dex.Resources
	}

	return resources
}

func getDexConfig(cr *argoproj.ArgoCD) string {
	config := common.ArgoCDDefaultDexConfig

	// Allow override of config from CR
	if cr.Spec.ExtraConfig["dex.config"] != "" {
		config = cr.Spec.ExtraConfig["dex.config"]
	} else if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && len(cr.Spec.SSO.Dex.Config) > 0 {
		config = cr.Spec.SSO.Dex.Config
	}
	return config
}

// dexServerCustomStartupScript returns the script that is required for generating dex config from `argocd-cm` config map,
// updating the dex storage to kubernetes and generate TLS certs and start the dex server.
func dexServerCustomStartupScript() []string {
	storageType := getDexStorageType()
	return []string{
		fmt.Sprintf(customBootstrapScriptTemplate, getDexStorageHealthCheckScriptlet(storageType), getDexStorageModificationScriptlet(storageType)),
	}
}

// getDexStorageType returns the storage type that needs to be used for dex.
func getDexStorageType() string {
	return defaultDexStorageType
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
