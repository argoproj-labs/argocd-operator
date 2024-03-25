package appcontroller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

const (
	redisTLSCertPath = "/app/config/controller/tls/redis/tls.crt"
)

func (acr *AppControllerReconciler) dynamicScalingEnabled() bool {
	return acr.Instance.Spec.Controller.Sharding.DynamicScalingEnabled != nil && *acr.Instance.Spec.Controller.Sharding.DynamicScalingEnabled
}

func (acr *AppControllerReconciler) staticScalingEnabled() bool {
	return acr.Instance.Spec.Controller.Sharding.Replicas != 0 && acr.Instance.Spec.Controller.Sharding.Enabled
}

func (acr *AppControllerReconciler) getStatusProcessors() int32 {
	sp := common.ArgoCDDefaultServerStatusProcessors
	if acr.Instance.Spec.Controller.Processors.Status > 0 {
		sp = acr.Instance.Spec.Controller.Processors.Status
	}
	return sp
}

func (acr *AppControllerReconciler) getOperationProcessors() int32 {
	op := common.ArgoCDDefaultServerOperationProcessors
	if acr.Instance.Spec.Controller.Processors.Operation > 0 {
		op = acr.Instance.Spec.Controller.Processors.Operation
	}
	return op
}

func (acr *AppControllerReconciler) getParallelismLimit() int32 {
	pl := common.ArgoCDDefaultControllerParallelismLimit
	if acr.Instance.Spec.Controller.ParallelismLimit > 0 {
		pl = acr.Instance.Spec.Controller.ParallelismLimit
	}
	return pl
}

func (acr *AppControllerReconciler) getResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}
	return argocdcommon.GetValueOrDefault(acr.Instance.Spec.Controller.Resources, resources).(corev1.ResourceRequirements)
}

func (acr *AppControllerReconciler) getCmd() []string {
	cmd := []string{
		"argocd-application-controller",
		"--operation-processors", fmt.Sprint(acr.getOperationProcessors()),
	}

	// redis flags
	if acr.Instance.Spec.Redis.IsEnabled() {
		cmd = append(cmd, common.RedisCmd, acr.Redis.GetServerAddress())
		if acr.Redis.UseTLS() {
			cmd = append(cmd, common.RedisUseTLSCmd)
			if acr.Instance.Spec.Redis.DisableTLSVerification {
				cmd = append(cmd, common.RedisInsecureSkipTLSVerifyCmd)
			} else {
				cmd = append(cmd, common.RedisCACertificate, redisTLSCertPath)
			}
		}
	} else {
		acr.Logger.Debug("redis is disabled; skipping redis configuration")
	}

	// repo-server flags
	if acr.Instance.Spec.Repo.IsEnabled() {
		cmd = append(cmd, "--repo-server", acr.RepoServer.GetServerAddress())
		if acr.RepoServer.TLSVerificationRequested() {
			cmd = append(cmd, "--repo-server-strict-tls")
		}
	} else {
		acr.Logger.Debug("repo server is disabled; skipping repo server configuration")
	}

	cmd = append(cmd, "--status-processors", fmt.Sprint(acr.getStatusProcessors()))
	cmd = append(cmd, "--kubectl-parallelism-limit", fmt.Sprint(acr.getParallelismLimit()))

	if acr.Instance.Spec.SourceNamespaces != nil && len(acr.Instance.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(acr.Instance.Spec.SourceNamespaces, ",")))
	}

	cmd = append(cmd, common.LogLevelCmd)
	cmd = append(cmd, argoutil.GetLogLevel(acr.Instance.Spec.Controller.LogLevel))

	cmd = append(cmd, common.LogFormatCmd)
	cmd = append(cmd, argoutil.GetLogFormat(acr.Instance.Spec.Controller.LogFormat))

	return cmd
}

func (acr *AppControllerReconciler) getContainerEnv() []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	env = append(env, corev1.EnvVar{
		Name:  "HOME",
		Value: "/home/argocd",
	})

	if acr.Instance.Spec.Controller.Sharding.Enabled {
		env = append(env, corev1.EnvVar{
			Name:  common.AppControllerReplicasEnvVar,
			Value: fmt.Sprint(acr.Instance.Spec.Controller.Sharding.Replicas),
		})
	}

	if acr.Instance.Spec.Controller.AppSync != nil {
		env = append(env, corev1.EnvVar{
			Name:  common.ArgoCDReconciliationTImeOutEnvVar,
			Value: strconv.FormatInt(int64(acr.Instance.Spec.Controller.AppSync.Seconds()), 10) + "s",
		})
	}

	return env
}

func (acr *AppControllerReconciler) getReplicaCount() int32 {
	var (
		replicas  int32 = common.ArgocdApplicationControllerDefaultReplicas
		minShards int32 = acr.Instance.Spec.Controller.Sharding.MinShards
		maxShards int32 = acr.Instance.Spec.Controller.Sharding.MaxShards
	)

	if acr.dynamicScalingEnabled() {
		// TODO: add the same validations to Validation Webhook once webhook has been introduced
		if minShards < 1 {
			acr.Logger.Debug("getReplicaCount: minimum number of shards cannot be less than 1; setting default value to 1")
			minShards = 1
		}

		if maxShards < minShards {
			acr.Logger.Debug("getReplicaCount: maximum number of shards cannot be less than minimum number of shards; setting maximum shards same as minimum shards")
			maxShards = minShards
		}

		clustersPerShard := acr.Instance.Spec.Controller.Sharding.ClustersPerShard
		if clustersPerShard < 1 {
			acr.Logger.Debug("getReplicaCount: clustersPerShard cannot be less than 1; defaulting to 1.")
			clustersPerShard = 1
		}

		clusterSecrets, err := argocdcommon.GetClusterSecrets(acr.Instance.Namespace, acr.Client)
		if err != nil {
			acr.Logger.Debug("getReplicaCount: failed to retrieve cluster secret list; using default value instead")
			return replicas
		}

		replicas = int32(len(clusterSecrets.Items)) / clustersPerShard

		if replicas < minShards {
			replicas = minShards
		}

		if replicas > maxShards {
			replicas = maxShards
		}
	} else if acr.staticScalingEnabled() {
		return acr.Instance.Spec.Controller.Sharding.Replicas
	}

	return replicas
}

func getCustomRoleName() string {
	return util.GetEnv(common.AppControllerClusterRoleEnvVar)
}

func (acr *AppControllerReconciler) getManagedRBACToBeDeleted() ([]types.NamespacedName, []types.NamespacedName, error) {
	roles := []types.NamespacedName{}
	rbs := []types.NamespacedName{}

	compReq, err := argocdcommon.GetComponentLabelRequirement(component)
	if err != nil {
		return nil, nil, err
	}

	rbacReq, err := argocdcommon.GetRbacTypeLabelRequirement(common.ArgoCDRBACTypeResourceMananagement)
	if err != nil {
		return nil, nil, err
	}

	ls := argocdcommon.GetLabelSelector(*compReq, *rbacReq)

	for ns := range acr.ManagedNamespaces {
		nsRoles, nsRbs := argocdcommon.GetRBACToBeDeleted(ns, ls, acr.Client, acr.Logger)
		roles = append(roles, nsRoles...)
		rbs = append(rbs, nsRbs...)
	}

	return roles, rbs, nil
}

func getAppControllerLabelSelector() (labels.Selector, error) {
	appControllerComponentReq, err := argocdcommon.GetLabelRequirements(common.AppK8sKeyComponent, selection.Equals, []string{common.AppControllerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getAppControllerLabelSelector: failed to generate label selector")
	}
	appControllerLS := argocdcommon.GetLabelSelector(*appControllerComponentReq)
	return appControllerLS, nil
}
