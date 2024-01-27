package redis

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	checksumInitConfigKey = "checksum/init-config"

	initShPath             = "/readonly-config/init.sh"
	readinessShPath        = "/health/redis_readiness.sh"
	livenessShPath         = "/health/redis_liveness.sh"
	redisConfPath          = "/data/conf/redis.conf"
	sentinelConfPath       = "/data/conf/sentinel.conf"
	sentinelLivenessShPath = "/health/sentinel_liveness.sh"
)

var (
	defaultMode                   int32 = 493
	terminationGracePeriodSeconds int64 = 60
	fsGroup                       int64 = 1000
	allowPrivilegeEscalation            = false
	runAsNonRoot                        = true
	runAsUser                     int64 = 1000

	failureThreshold    int32 = 5
	initialDelaySeconds int32 = 30
	periodSeconds       int32 = 15
	successThreshold    int32 = 1
	timeoutSeconds      int32 = 15
)

func (rr *RedisReconciler) reconcileHAStatefulSet() error {
	ssReq := rr.getStatefulSetRequest()

	desiredSS, err := workloads.RequestStatefulSet(ssReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileHAStatefulSet: failed to reconcile statefulset %s", desiredSS.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredSS, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAStatefulSet: failed to set owner reference for statefulset", "name", desiredSS.Name, "namespace", desiredSS.Namespace)
	}

	existingSS, err := workloads.GetStatefulSet(desiredSS.Name, desiredSS.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHAStatefulSet: failed to retrieve statefulset %s", desiredSS.Name)
		}

		if err = workloads.CreateStatefulSet(desiredSS, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHAStatefulSet: failed to create statefulset %s in namespace %s", desiredSS.Name, desiredSS.Namespace)
		}
		rr.Logger.Info("statefulset created", "name", desiredSS.Name, "namespace", desiredSS.Namespace)
		return nil
	}

	ssChanged := false

	for i, _ := range existingSS.Spec.Template.Spec.Containers {

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{&existingSS.Spec.Template.Spec.Containers[i].Image, &desiredSS.Spec.Template.Spec.Containers[i].Image,
				func() {
					existingSS.Spec.Template.ObjectMeta.Labels[common.ImageUpgradedKey] = time.Now().UTC().Format(common.TimeFormatMST)
				},
			},
			{&existingSS.Spec.Template.Spec.Containers[i].Resources, &desiredSS.Spec.Template.Spec.Containers[i].Resources, nil},
			{&existingSS.Spec.Template.Spec.SecurityContext, &desiredSS.Spec.Template.Spec.SecurityContext, nil},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &ssChanged)
		}
	}

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingSS.Spec.Template.Spec.InitContainers[0].Resources, &desiredSS.Spec.Template.Spec.InitContainers[0].Resources, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &ssChanged)
	}

	if !ssChanged {
		return nil
	}

	if err = workloads.UpdateStatefulSet(existingSS, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHAStatefulSet: failed to update statefulset %s", existingSS.Name)
	}

	rr.Logger.Info("statefulset updated", "name", existingSS.Name, "namespace", existingSS.Namespace)
	return nil
}

func (rr *RedisReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, rr.Client)
}

func (rr *RedisReconciler) deleteStatefulSet(name, namespace string) error {
	if err := workloads.DeleteStatefulSet(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteStatefulSet: failed to delete stateful set %s", name)
	}
	rr.Logger.Info("stateful set deleted", "name", name, "namespace", namespace)
	return nil
}

func (rr *RedisReconciler) getStatefulSetRequest() workloads.StatefulSetRequest {
	ssReq := workloads.StatefulSetRequest{
		ObjectMeta: argoutil.GetObjMeta(HAServerResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy: appsv1.OrderedReadyPodManagement,
			Replicas:            getHAReplicas(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: HAServerResourceName,
				},
			},
			ServiceName: HAResourceName,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						// TODO: Should this be hard-coded?
						checksumInitConfigKey: "7128bfbb51eafaffe3c33b1b463e15f0cf6514cec570f9d9c4f2396f28c724ac",
					},
					Labels: map[string]string{
						common.AppK8sKeyName: HAServerResourceName,
					},
				},
				Spec: rr.getStatefulSetPodSpec(),
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
		Instance:  rr.Instance,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Client:    rr.Client,
	}

	return ssReq
}

func (rr *RedisReconciler) getStatefulSetPodSpec() corev1.PodSpec {
	podspec := &corev1.PodSpec{}

	podspec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						common.AppK8sKeyName: HAResourceName,
					},
				},
				TopologyKey: common.K8sKeyHostname,
			}},
		},
	}

	podspec.AutomountServiceAccountToken = util.BoolPtr(false)
	podspec.Containers = rr.getStatefulSetContainers()
	podspec.InitContainers = rr.getStatefulSetInitContainer()
	podspec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup:      &fsGroup,
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}
	podspec.ServiceAccountName = resourceName
	podspec.TerminationGracePeriodSeconds = util.Int64Ptr(terminationGracePeriodSeconds)
	podspec.Volumes = getStatefulSetVolumes()

	return *podspec
}

func (rr *RedisReconciler) getStatefulSetContainers() []corev1.Container {
	containers := []corev1.Container{}

	redisContainer := corev1.Container{
		Name: redisName,
		Args: []string{
			redisConfPath,
		},
		Command: []string{
			"redis-server",
		},
		Image:           rr.getHAContainerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{{
			ContainerPort: common.DefaultRedisPort,
			Name:          redisName,
		}},
		LivenessProbe:  getStatefulSetProbe(),
		ReadinessProbe: getStatefulSetProbe(),
		Resources:      rr.getHAResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(true),
		},
		VolumeMounts: getStatefulSetContainerVolumeMounts(),
	}
	redisContainer.LivenessProbe.ProbeHandler.Exec.Command = getLivenessProbeCmd()
	redisContainer.ReadinessProbe.ProbeHandler.Exec.Command = getReadinessProbeCmd()

	sentinelContainer := corev1.Container{
		Name: sentinelName,
		Args: []string{
			sentinelConfPath,
		},
		Command: []string{
			"redis-sentinel",
		},
		Image:           rr.getHAContainerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports: []corev1.ContainerPort{{
			ContainerPort: common.DefaultRedisSentinelPort,
			Name:          sentinelName,
		}},
		LivenessProbe:  getStatefulSetProbe(),
		ReadinessProbe: getStatefulSetProbe(),
		Resources:      rr.getHAResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(runAsNonRoot),
		},
		VolumeMounts: getStatefulSetContainerVolumeMounts(),
	}

	sentinelContainer.LivenessProbe.ProbeHandler.Exec.Command = getSentinelProbeCmd()
	sentinelContainer.ReadinessProbe.ProbeHandler.Exec.Command = getSentinelProbeCmd()

	containers = append(containers, redisContainer, sentinelContainer)
	return containers
}

func (rr *RedisReconciler) getStatefulSetInitContainer() []corev1.Container {
	containers := []corev1.Container{}

	initc := corev1.Container{
		Args: []string{
			initShPath,
		},
		Command: []string{
			"sh",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SENTINEL_ID_0",
				Value: "3c0d9c0320bb34888c2df5757c718ce6ca992ce6", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "40000915ab58c3fa8fd888fb8b24711944e6cbb4", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "2bbec7894d954a8af3bb54d13eaec53cb024e2ca", // TODO: Should this be hard-coded?
			},
		},
		Image:           rr.getHAContainerImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Resources:       rr.getHAResources(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: util.BoolPtr(allowPrivilegeEscalation),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: util.BoolPtr(runAsNonRoot),
		},
		VolumeMounts: getStatefulSetInitContainerVolumeMounts(),
	}

	containers = append(containers, initc)
	return containers
}

func getStatefulSetContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/data",
			Name:      "data",
		},
		{
			MountPath: "/health",
			Name:      "health",
		},
		{
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: TLSPath,
		},
	}
}

func getStatefulSetInitContainerVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			MountPath: "/readonly-config",
			Name:      "config",
			ReadOnly:  true,
		},
		{
			MountPath: "/data",
			Name:      "data",
		},
		{
			Name:      common.ArgoCDRedisServerTLSSecretName,
			MountPath: TLSPath,
		},
	}
}

func getStatefulSetVolumes() []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		},
		{
			Name: "health",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: util.Int32Ptr(defaultMode),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAHealthConfigMapName,
					},
				},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
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
}

func getStatefulSetProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{},
		},
		FailureThreshold:    failureThreshold,
		InitialDelaySeconds: initialDelaySeconds,
		PeriodSeconds:       periodSeconds,
		SuccessThreshold:    successThreshold,
		TimeoutSeconds:      timeoutSeconds,
	}
}

func getLivenessProbeCmd() []string {
	return []string{
		"sh",
		"-c",
		livenessShPath,
	}
}

func getReadinessProbeCmd() []string {
	return []string{
		"sh",
		"-c",
		readinessShPath,
	}
}

func getSentinelProbeCmd() []string {
	return []string{
		"sh",
		"-c",
		sentinelLivenessShPath,
	}
}
