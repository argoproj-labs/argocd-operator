package reposerver

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/redis"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// GetRepoServerResources will return the ResourceRequirements for the Argo CD Repo server container.
func (rsr *RepoServerReconciler) GetRepoServerResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if rsr.Instance.Spec.Repo.Resources != nil {
		resources = *rsr.Instance.Spec.Repo.Resources
	}

	return resources
}

// GetArgoRepoServerCommand will return the command for the ArgoCD Repo component.
func (rsr *RepoServerReconciler) GetArgoRepoServerCommand(useTLSForRedis bool) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, UidEntryPointSh)
	cmd = append(cmd, RepoServerController)

	cmd = append(cmd, redis.Redis)
	cmd = append(cmd, getRedisServerAddress(rsr.Instance))

	if useTLSForRedis {
		cmd = append(cmd, redis.RedisUseTLS)
		if rsr.Instance.Spec.Redis.DisableTLSVerification {
			cmd = append(cmd, redis.RedisInsecureSkipTLSVerify)
		} else {
			cmd = append(cmd, redis.RedisCACertificate, RepoServerTLSRedisCertPath)
		}
	}

	cmd = append(cmd, LogLevel)
	cmd = append(cmd, util.GetLogLevel(rsr.Instance.Spec.Repo.LogLevel))

	cmd = append(cmd, LogFormat)
	cmd = append(cmd, util.GetLogFormat(rsr.Instance.Spec.Repo.LogFormat))

	// *** NOTE ***
	// Do Not add any new default command line arguments below this.
	extraArgs := rsr.Instance.Spec.Repo.ExtraRepoCommandArgs
	err := util.IsMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)
	return cmd
}

func (rsr *RepoServerReconciler) GetArgoCDRepoServerReplicas() *int32 {
	if rsr.Instance.Spec.Repo.Replicas != nil && *rsr.Instance.Spec.Repo.Replicas >= 0 {
		return rsr.Instance.Spec.Repo.Replicas
	}

	return nil
}

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func getRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}
	return util.FqdnServiceRef(common.ArgoCDDefaultRedisSuffix, cr.Namespace, common.ArgoCDDefaultRedisPort)
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *argoproj.ArgoCD) string {
	return util.FqdnServiceRef(redis.RedisHAProxyServiceName, cr.Namespace, common.ArgoCDDefaultRedisPort)
}
