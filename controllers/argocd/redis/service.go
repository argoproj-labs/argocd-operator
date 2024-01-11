package redis

import (
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (rr *RedisReconciler) reconcileHAProxyService() error {
	svcRequest := networking.ServiceRequest{
		ObjectMeta: metav1.ObjectMeta{},
	}
}
