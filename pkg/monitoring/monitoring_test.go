package monitoring

import (
	"errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *monitoringv1.PrometheusRule:
		obj.Name = test.TestNameMutated
		return nil
	case *monitoringv1.ServiceMonitor:
		obj.Name = test.TestNameMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
