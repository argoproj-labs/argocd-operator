// Copyright 2019 Argo CD Operator Developers
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
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// DexConnector represents an authentication connector for Dex.
type DexConnector struct {
	Config map[string]interface{} `yaml:"config,omitempty"`
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
}

// getDexContainerImage will return the container image for the Dex server.
func getDexContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.Dex.Image
	if len(img) <= 0 {
		img = common.ArgoCDDefaultDexImage
	}

	tag := cr.Spec.Dex.Version
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultDexVersion
	}
	return common.CombineImageTag(img, tag)
}

// getDexInitContainers will return the init-containers for the Dex server.
func getDexInitContainers(cr *v1alpha1.ArgoCD) []corev1.Container {
	return []corev1.Container{{
		Command: []string{
			"cp",
			"/usr/local/bin/argocd-util",
			"/shared",
		},
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}
}

// getDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func getDexOAuthClientID(cr *v1alpha1.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, common.ArgoCDDefaultDexServiceAccountName)
}

// getDexOAuthClientID will return the OAuth client secret for the given ArgoCD.
func (r *ArgoClusterReconciler) getDexOAuthClientSecret(cr *v1alpha1.ArgoCD) (*string, error) {
	sa := resources.NewServiceAccountWithName(cr.ObjectMeta, common.ArgoCDDefaultDexServiceAccountName)
	if err := resources.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	// Find the token secret
	var tokenSecret *corev1.ObjectReference
	for _, saSecret := range sa.Secrets {
		if strings.Contains(saSecret.Name, "token") {
			tokenSecret = &saSecret
			break
		}
	}

	if tokenSecret == nil {
		return nil, errors.New("unable to locate ServiceAccount token for OAuth client secret")
	}

	// Fetch the secret to obtain the token
	secret := resources.NewSecretWithName(cr.ObjectMeta, tokenSecret.Name)
	if err := resources.FetchObject(r.Client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	return &token, nil
}

// getDexOAuthRedirectURI will return the OAuth redirect URI for the Dex server.
func (r *ArgoClusterReconciler) getDexOAuthRedirectURI(cr *v1alpha1.ArgoCD) string {
	uri := r.getArgoServerURI(cr)
	return uri + common.ArgoCDDefaultDexOAuthRedirectPath
}

// getDexServerAddress will return the Dex server address.
func getDexServerAddress(cr *v1alpha1.ArgoCD) string {
	return fmt.Sprintf("http://%s:%d", common.NameWithSuffix(cr.ObjectMeta, "dex-server"), common.ArgoCDDefaultDexHTTPPort)
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ArgoClusterReconciler) getOpenShiftDexConfig(cr *v1alpha1.ArgoCD) (string, error) {
	clientSecret, err := r.getDexOAuthClientSecret(cr)
	if err != nil {
		return "", err
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     getDexOAuthClientID(cr),
			"clientSecret": *clientSecret,
			"redirectURI":  r.getDexOAuthRedirectURI(cr),
			"insecureCA":   true, // TODO: Configure for openshift CA
		},
	}

	connectors := make([]DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

// getDexResources will return the ResourceRequirements for the Dex container.
func getDexResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Dex.Resources != nil {
		resources = *cr.Spec.Dex.Resources
	}

	return resources
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ArgoClusterReconciler) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *v1alpha1.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)
	if len(desired) <= 0 && cr.Spec.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		desired = cfg
	}

	if actual != desired {
		// Update ConfigMap with desired configuration.
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		if err := r.Client.Update(context.TODO(), cm); err != nil {
			return err
		}

		// Trigger rollout of Dex Deployment to pick up changes.
		deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "dex-server", "dex-server")
		if !resources.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
			log.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.ObjectMeta.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		return r.Client.Update(context.TODO(), deploy)
	}
	return nil
}

// reconcileDexDeployment will ensure the Deployment resource is present for the ArgoCD Dex component.
func (r *ArgoClusterReconciler) reconcileDexDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "dex-server", "dex-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		actualImage := deploy.Spec.Template.Spec.Containers[0].Image
		desiredImage := getDexContainerImage(cr)
		if actualImage != desiredImage {
			deploy.Spec.Template.Spec.Containers[0].Image = desiredImage
			deploy.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Command: []string{
			"/shared/argocd-util",
			"rundex",
		},
		Image:           getDexContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "dex",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultDexHTTPPort,
				Name:          "http",
			}, {
				ContainerPort: common.ArgoCDDefaultDexGRPCPort,
				Name:          "grpc",
			},
		},
		Resources: getDexResources(cr),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Command: []string{
			"cp",
			"/usr/local/bin/argocd-util",
			"/shared",
		},
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = common.ArgoCDDefaultDexServiceAccountName
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{{
		Name: "static-files",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}}

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileDexService will ensure that the Service for Dex is present.
func (r *ArgoClusterReconciler) reconcileDexService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "dex-server", "dex-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "dex-server"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       common.ArgoCDDefaultDexHTTPPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexHTTPPort),
		}, {
			Name:       "grpc",
			Port:       common.ArgoCDDefaultDexGRPCPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultDexGRPCPort),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}

// reconcileDexServiceAccount will ensure that the Dex ServiceAccount is configured properly for OpenShift OAuth.
func (r *ArgoClusterReconciler) reconcileDexServiceAccount(cr *v1alpha1.ArgoCD) error {
	if !cr.Spec.Dex.OpenShiftOAuth {
		return nil // OpenShift OAuth not enabled, move along...
	}

	log.Info("oauth enabled, configuring dex service account")
	sa := resources.NewServiceAccountWithName(cr.ObjectMeta, common.ArgoCDDefaultDexServiceAccountName)
	if err := resources.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		return err
	}

	// Get the OAuth redirect URI that should be used.
	uri := r.getDexOAuthRedirectURI(cr)
	log.Info(fmt.Sprintf("URI: %s", uri))

	// Get the current redirect URI
	ann := sa.ObjectMeta.Annotations
	currentURI, found := ann[common.ArgoCDKeyDexOAuthRedirectURI]
	if found && currentURI == uri {
		return nil // Redirect URI annotation found and correct, move along...
	}

	log.Info(fmt.Sprintf("current URI: %s is not correct, should be: %s", currentURI, uri))
	if len(ann) <= 0 {
		ann = make(map[string]string)
	}

	ann[common.ArgoCDKeyDexOAuthRedirectURI] = uri
	sa.ObjectMeta.Annotations = ann

	return r.Client.Update(context.TODO(), sa)
}

// reconcileStatusDex will ensure that the Dex status is updated for the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileStatusDex(cr *v1alpha1.ArgoCD) error {
	status := "Unknown"

	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "dex-server", "dex-server")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		status = "Pending"

		if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
			status = "Running"
		}
	}

	if cr.Status.Dex != status {
		cr.Status.Dex = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
