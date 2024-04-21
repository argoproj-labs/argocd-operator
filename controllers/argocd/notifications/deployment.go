package notifications

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

func (nr *NotificationsReconciler) reconcileDeployment() error {
	desired := nr.getdesired()
	req := nr.getDeploymentRequest(*desired)

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.Deployment, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image,
				ExtraAction: func() {
					existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Env, Desired: &desired.Spec.Template.Spec.Containers[0].Env, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
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

	return nr.reconDeployment(req, argocdcommon.UpdateFnDep(updateFn), ignoreDrift)

}

func (nr *NotificationsReconciler) reconDeployment(req workloads.DeploymentRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestDeployment(req)
	if err != nil {
		nr.Logger.Debug("reconDeployment: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconDeployment: failed to request Deployment %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
		nr.Logger.Error(err, "reconDeployment: failed to set owner reference for Deployment", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetDeployment(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconDeployment: failed to retrieve Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateDeployment(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconDeployment: failed to create Deployment %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("Deployment created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = workloads.UpdateDeployment(existing, nr.Client); err != nil {
		return errors.Wrapf(err, "reconDeployment: failed to update Deployment %s", existing.Name)
	}

	nr.Logger.Info("Deployment updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (nr *NotificationsReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteDeployment: failed to delete deployment", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.Info("deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (nr *NotificationsReconciler) getdesired() *appsv1.Deployment {
	desired := &appsv1.Deployment{}

	notificationEnv := nr.Instance.Spec.Notifications.Env
	notificationEnv = util.EnvMerge(notificationEnv, util.ProxyEnvVars(), false)

	objMeta := argoutil.GetObjMeta(resourceName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap())
	podSpec := corev1.PodSpec{
		ServiceAccountName: resourceName,
		Volumes: []corev1.Volume{
			{
				Name: common.TLSCerts,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: common.ArgoCDTLSCertsConfigMapName,
						},
					},
				},
			},
			{
				Name: common.ArgoCDRepoServerTLS,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: common.ArgoCDRepoServerTLSSecretName,
						Optional:   util.BoolPtr(true),
					},
				},
			},
		},
		Containers: []corev1.Container{{
			Command:         nr.GetNotificationsCommand(),
			Image:           argocdcommon.GetArgoContainerImage(nr.Instance),
			ImagePullPolicy: corev1.PullAlways,
			Name:            common.NotificationsControllerComponent,
			Env:             notificationEnv,
			Resources:       nr.GetNotificationsResources(),
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{
							IntVal: int32(9001),
						},
					},
				},
			},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: util.BoolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						common.CapabilityDropAll,
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      common.TLSCerts,
					MountPath: common.VolumeMountPathTLS,
				},
				{
					Name:      common.ArgoCDRepoServerTLS,
					MountPath: common.VolumeMountPathRepoServerTLS,
				},
			},
			WorkingDir: common.WorkingDirApp,
		}},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: util.BoolPtr(true),
		},
	}

	deploymentSpec := appsv1.DeploymentSpec{
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
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
		Replicas: nr.GetArgoCDNotificationsControllerReplicas(),
	}

	desired.ObjectMeta = objMeta
	desired.Spec = deploymentSpec
	return desired
}

func (nr *NotificationsReconciler) getDeploymentRequest(dep appsv1.Deployment) workloads.DeploymentRequest {
	deploymentReq := workloads.DeploymentRequest{
		ObjectMeta: dep.ObjectMeta,
		Spec:       dep.Spec,
		Client:     nr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	return deploymentReq
}
