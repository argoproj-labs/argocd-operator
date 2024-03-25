package appcontroller

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

func (acr *AppControllerReconciler) reconcileStatefulSet() error {
	req := acr.getStatefulSetRequest()

	ignoreDrift := false
	updateFn := func(existing, desired *appsv1.StatefulSet, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Spec.Template.Spec.Containers[0].Image, Desired: &desired.Spec.Template.Spec.Containers[0].Image,
				ExtraAction: func() {
					if existing.Spec.Template.ObjectMeta.Labels == nil {
						existing.Spec.Template.ObjectMeta.Labels = map[string]string{}
					}
					existing.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Command, Desired: &desired.Spec.Template.Spec.Containers[0].Command, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Env, Desired: &desired.Spec.Template.Spec.Containers[0].Env, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].VolumeMounts, Desired: &desired.Spec.Template.Spec.Containers[0].VolumeMounts, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Containers[0].Resources, Desired: &desired.Spec.Template.Spec.Containers[0].Resources, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.NodeSelector, Desired: &desired.Spec.Template.Spec.NodeSelector, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Tolerations, Desired: &desired.Spec.Template.Spec.Tolerations, ExtraAction: nil},
			{Existing: &existing.Spec.Replicas, Desired: &desired.Spec.Replicas, ExtraAction: nil},
			{Existing: &existing.Spec.Template.Spec.Volumes, Desired: &desired.Spec.Template.Spec.Volumes, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}

	return acr.reconStatefulSet(req, argocdcommon.UpdateFnSs(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconStatefulSet(req workloads.StatefulSetRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := workloads.RequestStatefulSet(req)
	if err != nil {
		acr.Logger.Debug("reconStatefulSet: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconStatefulSet: failed to request StatefulSet %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(acr.Instance, desired, acr.Scheme); err != nil {
		acr.Logger.Error(err, "reconStatefulSet: failed to set owner reference for StatefulSet", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := workloads.GetStatefulSet(desired.Name, desired.Namespace, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconStatefulSet: failed to retrieve StatefulSet %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = workloads.CreateStatefulSet(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconStatefulSet: failed to create StatefulSet %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("StatefulSet created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// StatefulSet found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSs); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconStatefulSet: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	appControllerLs, err := getAppControllerLabelSelector()
	if err != nil {
		acr.Logger.Error(err, "failed to generate label selector")
	}
	invalidImagePod := argocdcommon.ContainsInvalidImage(appControllerLs, acr.Instance, acr.Client, acr.Logger)
	if invalidImagePod {
		if err := workloads.DeleteStatefulSet(resourceName, acr.Instance.Namespace, acr.Client); err != nil {
			return err
		}
	}

	if !changed {
		return nil
	}

	if err = workloads.UpdateStatefulSet(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconStatefulSet: failed to update StatefulSet %s", existing.Name)
	}

	acr.Logger.Info("StatefulSet updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (acr *AppControllerReconciler) deleteStatefulSet(name, namespace string) error {
	if err := workloads.DeleteStatefulSet(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteStatefulSet: failed to delete stateful set %s", name)
	}
	acr.Logger.Info("stateful set deleted", "name", name, "namespace", namespace)
	return nil
}

func (acr *AppControllerReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, acr.Client)
}

func (acr *AppControllerReconciler) getStatefulSetRequest() workloads.StatefulSetRequest {

	ssReq := workloads.StatefulSetRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, acr.Instance.Namespace, acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Spec: appsv1.StatefulSetSpec{
			Replicas: util.Int32Ptr(acr.getReplicaCount()),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.AppK8sKeyName: resourceName,
					},
				},
				Spec: acr.getStatefulSetPodSpec(),
			},
			ServiceName: resourceName,
		},
		Instance:  acr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    acr.Client,
	}

	return ssReq
}

func (acr *AppControllerReconciler) getStatefulSetPodSpec() corev1.PodSpec {
	podspec := &corev1.PodSpec{}

	podspec.Containers = acr.getPodspecContainers()
	podspec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: resourceName,
						},
					},
					TopologyKey: common.K8sKeyHostname,
				},
				Weight: int32(100),
			},
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.AppK8sKeyPartOf: common.ArgoCDAppName,
							},
						},
						TopologyKey: common.K8sKeyHostname,
					},
					Weight: int32(5),
				}},
		},
	}

	podspec.Volumes = []corev1.Volume{
		{
			Name: common.ArgoCDRepoServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   util.BoolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   util.BoolPtr(true),
				},
			},
		},
	}
	podspec.ServiceAccountName = resourceName
	podspec.NodeSelector = common.DefaultNodeSelector()

	if acr.Instance.Spec.NodePlacement != nil {
		podspec.NodeSelector = util.MergeMaps(podspec.NodeSelector, acr.Instance.Spec.NodePlacement.NodeSelector)
		podspec.Tolerations = acr.Instance.Spec.NodePlacement.Tolerations
	}

	// Handle import/restore from ArgoCDExport
	export, err := argocdcommon.GetArgoCDExport(acr.Instance, acr.Client)
	if err != nil {
		acr.Logger.Error(err, "getStatefulSetRequest: failed to retrieve Argo CD Export")
	}

	if export == nil {
		acr.Logger.Debug("getStatefulSetRequest: no existing export found; skipping import")
	} else {
		podspec.InitContainers = []corev1.Container{{
			Command:         argocdcommon.GetArgoImportCommand(acr.Client, acr.Instance),
			Env:             util.ProxyEnvVars(argocdcommon.GetArgoImportContainerEnv(export)...),
			Resources:       acr.getResources(),
			Image:           argocdcommon.GetArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: util.BoolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: util.BoolPtr(true),
			},
			VolumeMounts: argocdcommon.GetArgoImportVolumeMounts(),
		}}
		podspec.Volumes = append(podspec.Volumes, argocdcommon.GetArgoImportVolumes(export)...)
	}

	return *podspec
}

func (acr *AppControllerReconciler) getPodspecContainers() []corev1.Container {
	controllerEnv := acr.Instance.Spec.Controller.Env
	// Sharding setting explicitly overrides a value set in the env
	controllerEnv = util.EnvMerge(controllerEnv, acr.getContainerEnv(), true)
	// Let user specify their own environment first
	controllerEnv = util.EnvMerge(controllerEnv, util.ProxyEnvVars(), false)

	return []corev1.Container{{
		Command:         acr.getCmd(),
		Image:           argocdcommon.GetArgoContainerImage(acr.Instance),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-application-controller",
		Env:             controllerEnv,
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8082,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8082),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		Resources: acr.getResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/controller/tls",
			},
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/controller/tls/redis",
			},
		},
	}}
}
