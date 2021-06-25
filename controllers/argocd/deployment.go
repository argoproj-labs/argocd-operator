// Copyright 2021 ArgoCD Operator Developers
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
	"fmt"
	"os"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewDeploymentWithSuffix returns a new Deployment instance for the given ArgoCD using the given suffix.
func NewDeploymentWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
	return newDeploymentWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

// newDeploymentWithName returns a new Deployment instance for the given ArgoCD using the given name.
func newDeploymentWithName(name string, component string, cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
	deploy := newDeployment(cr)
	deploy.ObjectMeta.Name = name

	lbls := deploy.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	deploy.ObjectMeta.Labels = lbls

	deploy.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
			},
		},
	}

	return deploy
}

// newDeployment returns a new Deployment instance for the given ArgoCD.
func newDeployment(cr *argoprojv1a1.ArgoCD) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// IsDexDisabled returns true if Dex is disabled
func IsDexDisabled() bool {
	if v := os.Getenv("DISABLE_DEX"); v != "" {
		return strings.ToLower(v) == "true"
	}
	return false
}

// ProxyEnvVars creates proxy arariables
func ProxyEnvVars(vars ...corev1.EnvVar) []corev1.EnvVar {
	result := []corev1.EnvVar{}
	for _, v := range vars {
		result = append(result, v)
	}
	proxyKeys := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}
	for _, p := range proxyKeys {
		if k, v := caseInsensitiveGetenv(p); k != "" {
			result = append(result, corev1.EnvVar{Name: k, Value: v})
		}
	}
	return result
}

func caseInsensitiveGetenv(s string) (string, string) {
	if v := os.Getenv(s); v != "" {
		return s, v
	}
	ls := strings.ToLower(s)
	if v := os.Getenv(ls); v != "" {
		return ls, v
	}
	return "", ""
}

// GetArgoRepoCommand will return the command for the ArgoCD Repo component.
func GetArgoRepoCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// GetArgoServerCommand will return the command for the ArgoCD server component.
func GetArgoServerCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if getArgoServerInsecure(cr) {
		cmd = append(cmd, "--insecure")
	}

	if isRepoServerTLSVerificationRequested(cr) {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, getDexServerAddress(cr))

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, getRepoServerAddress(cr))

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	return cmd
}

// getDexServerAddress will return the Dex server address.
func getDexServerAddress(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("http://%s", fqdnServiceRef("dex-server", common.ArgoCDDefaultDexHTTPPort, cr))
}

// getRepoServerAddress will return the Argo CD repo server address.
func getRepoServerAddress(cr *argoprojv1a1.ArgoCD) string {
	return fqdnServiceRef("repo-server", common.ArgoCDDefaultRepoServerPort, cr)
}
