package reposerver

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-repo-server")

	cmd = append(cmd, "--redis")
	cmd = append(cmd, getRedisServerAddress(cr))

	if useTLSForRedis {
		cmd = append(cmd, "--redis-use-tls")
		if isRedisTLSVerificationDisabled(cr) {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/reposerver/tls/redis/tls.crt")
		}
	}

	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, getLogLevel(cr.Spec.Repo.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, getLogFormat(cr.Spec.Repo.LogFormat))

	// *** NOTE ***
	// Do Not add any new default command line arguments below this.
	extraArgs := cr.Spec.Repo.ExtraRepoCommandArgs
	err := isMergable(extraArgs, cmd)
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
