package argocd

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileMetricsService will ensure that the Service for the Argo CD application controller metrics is present.
func (r *ReconcileArgoCD) reconcileMetricsService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("metrics", "metrics", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		// Service found, do nothing
		return nil
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("application-controller", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8082,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8082),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

func (r *ReconcileArgoCD) getArgoCDExport(cr *argoproj.ArgoCD) *argoprojv1alpha1.ArgoCDExport {
	if cr.Spec.Import == nil {
		return nil
	}

	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	if argoutil.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export) {
		return export
	}
	return nil
}

func getArgoExportSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := argoutil.NameWithSuffix(export.ObjectMeta.Name, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

func getArgoImportBackend(client client.Client, cr *argoproj.ArgoCD) string {
	backend := common.ArgoCDExportStorageBackendLocal
	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import != nil && cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &argoprojv1alpha1.ArgoCDExport{}
	if argoutil.IsObjectFound(client, namespace, cr.Spec.Import.Name, export) {
		if export.Spec.Storage != nil && len(export.Spec.Storage.Backend) > 0 {
			backend = export.Spec.Storage.Backend
		}
	}
	return backend
}

// getArgoImportCommand will return the command for the ArgoCD import process.
func getArgoImportCommand(client client.Client, cr *argoproj.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-operator-util")
	cmd = append(cmd, "import")
	cmd = append(cmd, getArgoImportBackend(client, cr))
	return cmd
}

func getArgoImportContainerEnv(cr *argoprojv1alpha1.ArgoCDExport) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	switch cr.Spec.Storage.Backend {
	case common.ArgoCDExportStorageBackendAWS:
		env = append(env, corev1.EnvVar{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argoutil.FetchStorageSecretName(cr),
					},
					Key: "aws.access.key.id",
				},
			},
		})

		env = append(env, corev1.EnvVar{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: argoutil.FetchStorageSecretName(cr),
					},
					Key: "aws.secret.access.key",
				},
			},
		})
	}

	return env
}

// getArgoImportContainerImage will return the container image for the Argo CD import process.
func getArgoImportContainerImage(cr *argoprojv1alpha1.ArgoCDExport) string {
	img := common.ArgoCDDefaultExportJobImage
	if len(cr.Spec.Image) > 0 {
		img = cr.Spec.Image
	}

	tag := common.ArgoCDDefaultExportJobVersion
	if len(cr.Spec.Version) > 0 {
		tag = cr.Spec.Version
	}

	return argoutil.CombineImageTag(img, tag)
}

// getArgoImportVolumeMounts will return the VolumneMounts for the given ArgoCDExport.
func getArgoImportVolumeMounts() []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0)

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "backup-storage",
		MountPath: "/backups",
	})

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "secret-storage",
		MountPath: "/secrets",
	})

	return mounts
}

// getArgoImportVolumes will return the Volumes for the given ArgoCDExport.
func getArgoImportVolumes(cr *argoprojv1alpha1.ArgoCDExport) []corev1.Volume {
	volumes := make([]corev1.Volume, 0)

	if cr.Spec.Storage != nil && cr.Spec.Storage.Backend == common.ArgoCDExportStorageBackendLocal {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cr.Name,
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	volumes = append(volumes, corev1.Volume{
		Name: "secret-storage",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: getArgoExportSecretName(cr),
			},
		},
	})

	return volumes
}

func (r *ReconcileArgoCD) reconcileApplicationControllerStatefulSet(cr *argoproj.ArgoCD, useTLSForRedis bool) error {

	replicas := r.getApplicationControllerReplicaCount(cr)

	ss := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	ss.Spec.Replicas = &replicas
	controllerEnv := cr.Spec.Controller.Env
	// Sharding setting explicitly overrides a value set in the env
	controllerEnv = argoutil.EnvMerge(controllerEnv, getArgoControllerContainerEnv(cr), true)
	// Let user specify their own environment first
	controllerEnv = argoutil.EnvMerge(controllerEnv, proxyEnvVars(), false)
	podSpec := &ss.Spec.Template.Spec
	podSpec.Containers = []corev1.Container{{
		Command:         getArgoApplicationControllerCommand(cr, useTLSForRedis),
		Image:           getArgoContainerImage(cr),
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
		Resources: getArgoApplicationControllerResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
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
	AddSeccompProfileForOpenShift(r.Client, podSpec)
	podSpec.ServiceAccountName = nameWithSuffix("argocd-application-controller", cr)
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRepoServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: nameWithSuffix("argocd-application-controller", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
				Weight: int32(100),
			},
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyPartOf: common.ArgoCDAppName,
							},
						},
						TopologyKey: common.ArgoCDKeyHostname,
					},
					Weight: int32(5),
				}},
		},
	}

	// Handle import/restore from ArgoCDExport
	export := r.getArgoCDExport(cr)
	if export == nil {
		log.Info("existing argocd export not found, skipping import")
	} else {
		podSpec.InitContainers = []corev1.Container{{
			Command:         getArgoImportCommand(r.Client, cr),
			Env:             proxyEnvVars(getArgoImportContainerEnv(export)...),
			Resources:       getArgoApplicationControllerResources(cr),
			Image:           getArgoImportContainerImage(export),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "argocd-import",
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: boolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: boolPtr(true),
			},
			VolumeMounts: getArgoImportVolumeMounts(),
		}}

		podSpec.Volumes = getArgoImportVolumes(export)
	}

	invalidImagePod := containsInvalidImage(cr, r)
	if invalidImagePod {
		if err := r.Client.Delete(context.TODO(), ss); err != nil {
			return err
		}
	}

	existing := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Controller.IsEnabled() {
			log.Info("Existing application controller found but should be disabled. Deleting Application Controller")
			// Delete existing deployment for Application Controller, if any ..
			return r.Client.Delete(context.TODO(), existing)
		}
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getArgoContainerImage(cr)
		changed := false
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		desiredCommand := getArgoApplicationControllerCommand(cr, useTLSForRedis)
		if isRepoServerTLSVerificationRequested(cr) {
			desiredCommand = append(desiredCommand, "--repo-server-strict-tls")
		}
		updateNodePlacementStateful(existing, ss, &changed)
		if !reflect.DeepEqual(desiredCommand, existing.Spec.Template.Spec.Containers[0].Command) {
			existing.Spec.Template.Spec.Containers[0].Command = desiredCommand
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			ss.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = ss.Spec.Template.Spec.Containers[0].Env
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Volumes, existing.Spec.Template.Spec.Volumes) {
			existing.Spec.Template.Spec.Volumes = ss.Spec.Template.Spec.Volumes
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[0].VolumeMounts,
			existing.Spec.Template.Spec.Containers[0].VolumeMounts) {
			existing.Spec.Template.Spec.Containers[0].VolumeMounts = ss.Spec.Template.Spec.Containers[0].VolumeMounts
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = ss.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}
		if !reflect.DeepEqual(ss.Spec.Replicas, existing.Spec.Replicas) {
			existing.Spec.Replicas = ss.Spec.Replicas
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // StatefulSet found with nothing to do, move along...
	}

	if !cr.Spec.Controller.IsEnabled() {
		log.Info("Application Controller disabled. Skipping starting application controller.")
		return nil
	}

	// Delete existing deployment for Application Controller, if any ..
	deploy := newDeploymentWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, deploy.Namespace, deploy.Name, deploy) {
		if err := r.Client.Delete(context.TODO(), deploy); err != nil {
			return err
		}
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

// reconcileStatefulSets will ensure that all StatefulSets are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatefulSets(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	if err := r.reconcileApplicationControllerStatefulSet(cr, useTLSForRedis); err != nil {
		return err
	}
	if err := r.reconcileRedisStatefulSet(cr); err != nil {
		return err
	}
	return nil
}

// to update nodeSelector and tolerations in reconciler
func updateNodePlacementStateful(existing *appsv1.StatefulSet, ss *appsv1.StatefulSet, changed *bool) {
	if !reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, ss.Spec.Template.Spec.NodeSelector) {
		existing.Spec.Template.Spec.NodeSelector = ss.Spec.Template.Spec.NodeSelector
		*changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, ss.Spec.Template.Spec.Tolerations) {
		existing.Spec.Template.Spec.Tolerations = ss.Spec.Template.Spec.Tolerations
		*changed = true
	}
}

// Returns true if a StatefulSet has pods in ErrImagePull or ImagePullBackoff state.
// These pods cannot be restarted automatially due to known kubernetes issue https://github.com/kubernetes/kubernetes/issues/67250
func containsInvalidImage(cr *argoproj.ArgoCD, r *ReconcileArgoCD) bool {

	brokenPod := false

	podList := &corev1.PodList{}
	listOption := client.MatchingLabels{common.ArgoCDKeyName: fmt.Sprintf("%s-%s", cr.Name, "application-controller")}

	if err := r.Client.List(context.TODO(), podList, listOption); err != nil {
		log.Error(err, "Failed to list Pods")
	}
	if len(podList.Items) > 0 {
		if len(podList.Items[0].Status.ContainerStatuses) > 0 {
			if podList.Items[0].Status.ContainerStatuses[0].State.Waiting != nil && (podList.Items[0].Status.ContainerStatuses[0].State.Waiting.Reason == "ImagePullBackOff" || podList.Items[0].Status.ContainerStatuses[0].State.Waiting.Reason == "ErrImagePull") {
				brokenPod = true
			}
		}
	}
	return brokenPod
}

func (r *ReconcileArgoCD) getApplicationControllerReplicaCount(cr *argoproj.ArgoCD) int32 {
	var replicas int32 = common.ArgocdApplicationControllerDefaultReplicas
	var minShards int32 = cr.Spec.Controller.Sharding.MinShards
	var maxShards int32 = cr.Spec.Controller.Sharding.MaxShards

	if cr.Spec.Controller.Sharding.DynamicScalingEnabled != nil && *cr.Spec.Controller.Sharding.DynamicScalingEnabled {

		// TODO: add the same validations to Validation Webhook once webhook has been introduced
		if minShards < 1 {
			log.Info("Minimum number of shards cannot be less than 1. Setting default value to 1")
			minShards = 1
		}

		if maxShards < minShards {
			log.Info("Maximum number of shards cannot be less than minimum number of shards. Setting maximum shards same as minimum shards")
			maxShards = minShards
		}

		clustersPerShard := cr.Spec.Controller.Sharding.ClustersPerShard
		if clustersPerShard < 1 {
			log.Info("clustersPerShard cannot be less than 1. Defaulting to 1.")
			clustersPerShard = 1
		}

		clusterSecrets, err := r.getClusterSecrets(cr)
		if err != nil {
			// If we were not able to query cluster secrets, return the default count of replicas (ArgocdApplicationControllerDefaultReplicas)
			log.Error(err, "Error retreiving cluster secrets for ArgoCD instance %s", cr.Name)
			return replicas
		}

		replicas = int32(len(clusterSecrets.Items)) / clustersPerShard

		if replicas < minShards {
			replicas = minShards
		}

		if replicas > maxShards {
			replicas = maxShards
		}

		return replicas

	} else if cr.Spec.Controller.Sharding.Replicas != 0 && cr.Spec.Controller.Sharding.Enabled {
		return cr.Spec.Controller.Sharding.Replicas
	}

	return replicas
}

func getArgoControllerContainerEnv(cr *argoproj.ArgoCD) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	env = append(env, corev1.EnvVar{
		Name:  "HOME",
		Value: "/home/argocd",
	})

	if cr.Spec.Controller.Sharding.Enabled {
		env = append(env, corev1.EnvVar{
			Name:  "ARGOCD_CONTROLLER_REPLICAS",
			Value: fmt.Sprint(cr.Spec.Controller.Sharding.Replicas),
		})
	}

	if cr.Spec.Controller.AppSync != nil {
		env = append(env, corev1.EnvVar{
			Name:  "ARGOCD_RECONCILIATION_TIMEOUT",
			Value: strconv.FormatInt(int64(cr.Spec.Controller.AppSync.Seconds()), 10) + "s",
		})
	}

	return env
}

func policyRuleForApplicationController() []v1.PolicyRule {

	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

// getArgoApplicationControllerResources will return the ResourceRequirements for the Argo CD application controller container.
func getArgoApplicationControllerResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Controller.Resources != nil {
		resources = *cr.Spec.Controller.Resources
	}

	return resources
}

// getArgoApplicationControllerCommand will return the command for the ArgoCD Application Controller component.
func getArgoApplicationControllerCommand(cr *argoproj.ArgoCD, useTLSForRedis bool) []string {
	cmd := []string{
		"argocd-application-controller",
		"--operation-processors", fmt.Sprint(getArgoServerOperationProcessors(cr)),
	}

	if cr.Spec.Redis.IsEnabled() {
		cmd = append(cmd, "--redis", getRedisServerAddress(cr))
	} else {
		log.Info("Redis is Disabled. Skipping adding Redis configuration to Application Controller.")
	}

	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/controller/tls/redis/tls.crt")
		}
	}

	if cr.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--repo-server", getRepoServerAddress(cr))
	} else {
		log.Info("Repo Server is disabled. This would affect the functioning of Application Controller.")
	}

	cmd = append(cmd, "--status-processors", fmt.Sprint(getArgoServerStatusProcessors(cr)))
	cmd = append(cmd, "--kubectl-parallelism-limit", fmt.Sprint(getArgoControllerParellismLimit(cr)))

	if cr.Spec.SourceNamespaces != nil && len(cr.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(cr.Spec.SourceNamespaces, ",")))
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Controller.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Controller.LogFormat))

	return cmd
}

// getArgoServerOperationProcessors will return the numeric Operation Processors value for the ArgoCD Server.
func getArgoServerOperationProcessors(cr *argoproj.ArgoCD) int32 {
	op := common.ArgoCDDefaultServerOperationProcessors
	if cr.Spec.Controller.Processors.Operation > 0 {
		op = cr.Spec.Controller.Processors.Operation
	}
	return op
}

// getArgoServerStatusProcessors will return the numeric Status Processors value for the ArgoCD Server.
func getArgoServerStatusProcessors(cr *argoproj.ArgoCD) int32 {
	sp := common.ArgoCDDefaultServerStatusProcessors
	if cr.Spec.Controller.Processors.Status > 0 {
		sp = cr.Spec.Controller.Processors.Status
	}
	return sp
}

// getArgoControllerParellismLimit returns the parallelism limit for the application controller
func getArgoControllerParellismLimit(cr *argoproj.ArgoCD) int32 {
	pl := common.ArgoCDDefaultControllerParallelismLimit
	if cr.Spec.Controller.ParallelismLimit > 0 {
		pl = cr.Spec.Controller.ParallelismLimit
	}
	return pl
}

// reconcileMetricsServiceMonitor will ensure that the ServiceMonitor is present for the ArgoCD metrics Service.
func (r *ReconcileArgoCD) reconcileMetricsServiceMonitor(cr *argoproj.ArgoCD) error {
	sm := newServiceMonitorWithSuffix(common.ArgoCDKeyMetrics, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, sm.Name, sm) {
		if !cr.Spec.Prometheus.Enabled {
			// ServiceMonitor exists but enabled flag has been set to false, delete the ServiceMonitor
			return r.Client.Delete(context.TODO(), sm)
		}
		return nil // ServiceMonitor found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	sm.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix(common.ArgoCDKeyMetrics, cr),
		},
	}
	sm.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: common.ArgoCDKeyMetrics,
		},
	}

	if err := controllerutil.SetControllerReference(cr, sm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), sm)
}

// reconcileStatusApplicationController will ensure that the ApplicationController Status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusApplicationController(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	ss := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
		status = "Pending"

		if ss.Spec.Replicas != nil {
			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
	}

	if cr.Status.ApplicationController != status {
		cr.Status.ApplicationController = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
