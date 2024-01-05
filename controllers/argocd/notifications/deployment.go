package notifications

import (
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileDeployment() error {

	nr.Logger.Info("reconciling deployment")

	desiredDeployment := nr.getDesiredDeployment()
	deploymentRequest := nr.getDeploymentRequest(*desiredDeployment)

	desiredDeployment, err := workloads.RequestDeployment(deploymentRequest)
	if err != nil {
		nr.Logger.Error(err, "reconcileDeployment: failed to request deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		nr.Logger.V(1).Info("reconcileDeployment: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileDeployment: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.deleteDeployment(desiredDeployment.Name, desiredDeployment.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileDeployment: failed to delete deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}
		return err
	}

	existingDeployment, err := workloads.GetDeployment(desiredDeployment.Name, desiredDeployment.Namespace, nr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileDeployment: failed to retrieve deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredDeployment, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileDeployment: failed to set owner reference for deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		}

		if err = workloads.CreateDeployment(desiredDeployment, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileDeployment: failed to create deployment", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
			return err
		}
		nr.Logger.V(0).Info("reconcileDeployment: deployment created", "name", desiredDeployment.Name, "namespace", desiredDeployment.Namespace)
		return nil
	}
	deploymentChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingDeployment.Spec.Template.Spec.Containers[0].Image, &desiredDeployment.Spec.Template.Spec.Containers[0].Image,
			func() {
				existingDeployment.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
			},
		},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Command, &desiredDeployment.Spec.Template.Spec.Containers[0].Command, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Env, &desiredDeployment.Spec.Template.Spec.Containers[0].Env, nil},
		{&existingDeployment.Spec.Template.Spec.Containers[0].Resources, &desiredDeployment.Spec.Template.Spec.Containers[0].Resources, nil},
		{&existingDeployment.Spec.Template.Spec.Volumes, &desiredDeployment.Spec.Template.Spec.Volumes, nil},
		{&existingDeployment.Spec.Template.Spec.NodeSelector, &desiredDeployment.Spec.Template.Spec.NodeSelector, nil},
		{&existingDeployment.Spec.Template.Spec.Tolerations, &desiredDeployment.Spec.Template.Spec.Tolerations, nil},
		{&existingDeployment.Spec.Template.Spec.ServiceAccountName, &desiredDeployment.Spec.Template.Spec.ServiceAccountName, nil},
		{&existingDeployment.Spec.Template.Labels, &desiredDeployment.Spec.Template.Labels, nil},
		{&existingDeployment.Spec.Replicas, &desiredDeployment.Spec.Replicas, nil},
		{&existingDeployment.Spec.Selector, &desiredDeployment.Spec.Selector, nil},
		{&existingDeployment.Labels, &desiredDeployment.Labels, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &deploymentChanged)
	}

	if deploymentChanged {

		if err = workloads.UpdateDeployment(existingDeployment, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileDeployment: failed to update deployment", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
			return err
		}
	}

	nr.Logger.V(0).Info("reconcileDeployment: deployment updated", "name", existingDeployment.Name, "namespace", existingDeployment.Namespace)
	return nil
}

func (nr *NotificationsReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, nr.Client); err != nil {
		nr.Logger.Error(err, "DeleteDeployment: failed to delete deployment", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.V(0).Info("DeleteDeployment: deployment deleted", "name", name, "namespace", namespace)
	return nil
}

func (nr *NotificationsReconciler) getDesiredDeployment() *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{}

	notificationEnv := nr.Instance.Spec.Notifications.Env
	notificationEnv = util.EnvMerge(notificationEnv, util.ProxyEnvVars(), false)

	objMeta := metav1.ObjectMeta{
		Name:      resourceName,
		Namespace: nr.Instance.Namespace,
		Labels:    resourceLabels,
	}
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

	desiredDeployment.ObjectMeta = objMeta
	desiredDeployment.Spec = deploymentSpec
	return desiredDeployment
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
