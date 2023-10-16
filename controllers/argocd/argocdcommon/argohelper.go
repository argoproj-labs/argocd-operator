package argocdcommon

import (
	"fmt"
	"os"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func GetArgoContainerImage(cr *argoproj.ArgoCD) string {
	defaultTag, defaultImg := false, false
	img := cr.Spec.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvVar); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return util.CombineImageTag(img, tag)
}

// getArgoCmpServerInitCommand will return the command for the ArgoCD CMP Server init container
func GetArgoCmpServerInitCommand() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "cp")
	cmd = append(cmd, "-n")
	cmd = append(cmd, "/usr/local/bin/argocd")
	cmd = append(cmd, "/var/run/argocd/argocd-cmp-server")
	return cmd
}

// isOwnerOfInterest returns true if the given owner is one of the Argo CD services that
// may have been made the owner of the tls secret created by the OpenShift service CA, used
// to secure communication amongst the Argo CD components.
func IsOwnerOfInterest(owner metav1.OwnerReference) bool {
	if owner.Kind != "Service" {
		return false
	}
	if strings.HasSuffix(owner.Name, "-repo-server") {
		return true
	}
	if strings.HasSuffix(owner.Name, "-redis") {
		return true
	}
	return false
}

// TriggerRollout will trigger a rollout of a Kubernetes resource specified as
// obj. It currently supports Deployment and StatefulSet resources.
func TriggerRollout(client cntrlClient.Client, name, namespace, resType string, opt func(name string, namespace string)) error {
	switch resType {
	case common.DeploymentKind:
		return workloads.TriggerDeploymentRollout(client, name, namespace, opt)
	case common.StatefulSetKind:
		return workloads.TriggerStatefulSetRollout(client, name, namespace, opt)
	default:
		return fmt.Errorf("resource of unknown type %T, cannot trigger rollout", resType)
	}
}
