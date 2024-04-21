package dex

import (
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileDeployment will ensure all ArgoCD dex Server deployment is present
func (dr *DexReconciler) reconcileDeployment() error {
	req := dr.getDeploymentReq()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image, ExtraAction: func() {
				existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
			},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Env, Desired: &desired.Spec.Template.Spec.Containers[0].Env, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Volumes, Desired: &desired.Spec.Template.Spec.Volumes, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.ServiceAccountName, Desired: &desired.Spec.Template.Spec.ServiceAccountName, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Labels, Desired: &desired.Spec.Template.Labels, ExtraAction: nil},
			{Existing: &existing.Spec.Replicas, Desired: &desired.Spec.Replicas, ExtraAction: nil},
			{Existing: &existing.Spec.Selector, Desired: &desired.Spec.Selector, ExtraAction: nil},
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return dr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)

}

func (dr *DexReconciler) reconDeployment(req workloads.DeploymentRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		dr.Logger.Debug("reconDeployment: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconDeployment: failed to request Deployment %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(dr.Instance, desired, dr.Scheme); err != nil {
		dr.Logger.Error(err, "reconDeployment: failed to set owner reference for Deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, dr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconDeployment: failed to retrieve Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateDeployment(desired, dr.Client); err != nil {
			return errors.Wrapf(err, "reconDeployment: failed to create Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		dr.Logger.Info("Deployment created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Deployment found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnDep); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconDeployment: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateDeployment(existing, dr.Client); err != nil {
		return errors.Wrapf(err, "reconDeployment: failed to update Deployment %s", existing.Name)
	}

	dr.Logger.Info("Deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteDeployment will delete deployment with given name.
func (dr *DexReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, dr.Client); err != nil {
		// resource is already deleted, ignore error
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s in namespace %s", name, namespace)
	}
	dr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

// TriggerDeploymentRollout starts server deployment rollout by updating the given key
func (dr *DexReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, dr.Client)
}

func (dr *DexReconciler) getDeploymentReq() workloads.DeploymentRequest {
	req := workloads.DeploymentRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, dr.Instance.Namespace, dr.Instance.Name, dr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Client:     dr.Client,
		Instance:   dr.Instance,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	dexEnv := util.ProxyEnvVars()
	if dr.Instance.Spec.SSO != nil && dr.Instance.Spec.SSO.Dex != nil {
		dexEnv = append(dexEnv, dr.Instance.Spec.SSO.Dex.Env...)
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes: []corev1.Volume{{
			Name: "static-files",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}},
		InitContainers: []corev1.Container{{
			Command: []string{
				"cp",
				"-n",
				"/usr/local/bin/argocd",
				"/shared/argocd-dex",
			},
			Env:             util.ProxyEnvVars(),
			Image:           argocdcommon.GetArgoContainerImage(dr.Instance),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "copyutil",
			Resources:       dr.getResources(),
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: util.BoolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: util.BoolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "static-files",
				MountPath: "/shared",
			}},
		}},
		Containers: []corev1.Container{{
			Command: []string{
				"/shared/argocd-dex",
				"rundex",
			},
			Image: dr.getContainerImage(),
			Name:  "dex",
			Env:   dexEnv,
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/healthz/live",
						Port: intstr.FromInt(common.ArgoCDDefaultDexMetricsPort),
					},
				},
				InitialDelaySeconds: 60,
				PeriodSeconds:       30,
			},
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: common.ArgoCDDefaultDexHTTPPort,
					Name:          "http",
				}, {
					ContainerPort: common.ArgoCDDefaultDexGRPCPort,
					Name:          "grpc",
				}, {
					ContainerPort: common.ArgoCDDefaultDexMetricsPort,
					Name:          "metrics",
				},
			},
			Resources: dr.getResources(),
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: util.BoolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: util.BoolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "static-files",
				MountPath: "/shared",
			}},
		}},
	}

	req.Spec = appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Template: corev1.PodTemplateSpec{
			Spec: podSpec,
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.AppK8sKeyName: resourceName,
			},
		},
	}

	return req
}
