package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
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

// GetRepoServerCommand will return the command for the ArgoCD Repo component.
func (rsr *RepoServerReconciler) GetRepoServerCommand(useTLSForRedis bool) []string {
	cmd := make([]string, 0)

	cmd = append(cmd, common.UidEntryPointSh)
	cmd = append(cmd, common.RepoServerController)

	cmd = append(cmd, common.Redis)
	cmd = append(cmd, argocdcommon.GetRedisServerAddress(rsr.Instance))

	if useTLSForRedis {
		cmd = append(cmd, common.RedisUseTLS)
		if rsr.Instance.Spec.Redis.DisableTLSVerification {
			cmd = append(cmd, common.RedisInsecureSkipTLSVerify)
		} else {
			cmd = append(cmd, common.RedisCACertificate, common.RepoServerTLSRedisCertPath)
		}
	}

	cmd = append(cmd, common.LogLevel)
	cmd = append(cmd, util.GetLogLevel(rsr.Instance.Spec.Repo.LogLevel))

	cmd = append(cmd, common.LogFormat)
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

func (rsr *RepoServerReconciler) GetRepoServerReplicas() *int32 {
	if rsr.Instance.Spec.Repo.Replicas != nil && *rsr.Instance.Spec.Repo.Replicas >= 0 {
		return rsr.Instance.Spec.Repo.Replicas
	}

	return nil
}

// GetRepoServerAddress will return the Argo CD repo server address.
func GetRepoServerAddress(name string, namespace string) string {
	return util.FqdnServiceRef(util.NameWithSuffix(name, common.RepoServerControllerComponent), namespace, common.ArgoCDDefaultRepoServerPort)
}

func (rsr *RepoServerReconciler) TriggerRepoServerDeploymentRollout() error {
	name := util.NameWithSuffix(rsr.Instance.Name, common.RepoServerControllerComponent)
	return workloads.TriggerDeploymentRollout(rsr.Client, name, rsr.Instance.Namespace, func(name string, namespace string) {
		deployment, err := workloads.GetDeployment(name, namespace, rsr.Client)
		if err != nil {
			rsr.Logger.Error(err, "triggerRepoServerDeploymentRollout: failed to trigger repo-server deployment", "name", name, "namespace", namespace)
		}
		if deployment.Spec.Template.ObjectMeta.Labels == nil {
			deployment.Spec.Template.ObjectMeta.Labels = make(map[string]string)
		}
		deployment.Spec.Template.ObjectMeta.Labels[common.ArgoCDRepoTLSCertChangedKey] = util.NowNano()
	})
}
