package monitoring

import (
	"errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// common test variables used across workloads tests
var (
	testName              = "test-name"
	testInstance          = "test-instance"
	testInstanceNamespace = "test-instance-ns"
	testNamespace         = "test-ns"
	testComponent         = "test-component"
	testKey               = "test-key"
	testVal               = "test-value"

	testPrometheusRuleNameMutated = "mutated-name"
	testServiceMonitorNameMutated = "mutated-name"
	testKVP                       = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	switch obj := resource.(type) {
	case *monitoringv1.PrometheusRule:
		obj.Name = testPrometheusRuleNameMutated
		return nil
	case *monitoringv1.ServiceMonitor:
		obj.Name = testServiceMonitorNameMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
