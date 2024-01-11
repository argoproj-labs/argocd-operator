package openshfit

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
)

// AddAutoTLSAnnotation adds the OpenShift Service CA TLS cert request annotaiton to the provided service request object, using the provided secret name as the value
func AddAutoTLSAnnotation(svcReq networking.ServiceRequest, secretName string) networking.ServiceRequest {
	// We currently only support OpenShift for automatic TLS
	if !cluster.IsOpenShiftEnv() {
		return svcReq
	}

	if svcReq.ObjectMeta.Annotations == nil {
		svcReq.ObjectMeta.Annotations = make(map[string]string)
	}

	svcReq.ObjectMeta.Annotations[common.ServiceBetaOpenshiftKeyCertSecret] = secretName
	return svcReq
}
