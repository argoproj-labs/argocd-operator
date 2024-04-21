package reposerver

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
)

const (
	redisTLSCertPath = "/app/config/reposerver/tls/redis/tls.crt"
)

func (rsr *RepoServerReconciler) TLSVerificationRequested() bool {
	return rsr.Instance.Spec.Repo.VerifyTLS
}

// UseTLS determines whether repo-server component should communicate with TLS or not
func (rsr *RepoServerReconciler) UseTLS() bool {
	rsr.TLSEnabled = argocdcommon.UseTLS(common.ArgoCDRepoServerTLS, rsr.Instance.Namespace, rsr.Client, rsr.Logger)
	// returning for interface compliance
	return rsr.TLSEnabled
}

// getResources will return the ResourceRequirements for the Repo server container.
func (rsr *RepoServerReconciler) getResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if rsr.Instance.Spec.Repo.Resources != nil {
		resources = *rsr.Instance.Spec.Repo.Resources
	}

	return resources
}

// getContainerImage will return the container image for the repo-server.
func (rsr *RepoServerReconciler) getContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.Repo.Image, cr.Spec.Repo.Version
	}
	return argocdcommon.GetContainerImage(fn, rsr.Instance, common.ArgoCDImageEnvVar, common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion)
}

// GetServerAddress will return the repo-server service address for the given ArgoCD instance
func (rsr *RepoServerReconciler) GetServerAddress() string {
	rsr.varSetter()
	if rsr.Instance.Spec.Repo.Remote != nil && *rsr.Instance.Spec.Repo.Remote != "" {
		return *rsr.Instance.Spec.Repo.Remote
	}

	return argoutil.FQDNwithPort(resourceName, rsr.Instance.Namespace, common.DefaultRepoServerPort)
}

// getReplicas will return the size value for the argocd-repo-server replica count if it
// has been set in argocd CR. Otherwise, nil is returned if the replicas is not set in the argocd CR or
// replicas value is < 0.
func (rsr *RepoServerReconciler) getReplicas() *int32 {
	if rsr.Instance.Spec.Repo.Replicas != nil && *rsr.Instance.Spec.Repo.Replicas >= 0 {
		return rsr.Instance.Spec.Repo.Replicas
	}

	return nil
}

// getArgs will return the args for the repo server container
func (rsr *RepoServerReconciler) getArgs() []string {
	cmd := make([]string, 0)

	cmd = append(cmd, common.UidEntryPointSh)
	cmd = append(cmd, common.RepoServerCmd)

	if rsr.Instance.Spec.Redis.IsEnabled() {
		cmd = append(cmd, common.RedisCmd, rsr.Redis.GetServerAddress())

		if rsr.Redis.UseTLS() {
			cmd = append(cmd, common.RedisUseTLSCmd)
			if rsr.Instance.Spec.Redis.DisableTLSVerification {
				cmd = append(cmd, common.RedisInsecureSkipTLSVerifyCmd)
			} else {
				cmd = append(cmd, common.RedisCACertificate, redisTLSCertPath)
			}
		}
	} else {
		rsr.Logger.Debug("redis is disabled; skipping redis configuration")
	}

	cmd = append(cmd, common.LogLevelCmd)
	cmd = append(cmd, argoutil.GetLogLevel(rsr.Instance.Spec.Repo.LogLevel))

	cmd = append(cmd, common.LogFormatCmd)
	cmd = append(cmd, argoutil.GetLogFormat(rsr.Instance.Spec.Repo.LogFormat))

	// *** NOTE ***
	// Do Not add any new default command line arguments below this.
	extraArgs := rsr.Instance.Spec.Repo.ExtraRepoCommandArgs
	err := argocdcommon.IsMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)
	return cmd
}
