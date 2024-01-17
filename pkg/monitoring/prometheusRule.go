package monitoring

import (
	"errors"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// PrometheusRuleRequest objects contain all the required information to produce a prometheusRule object in return
type PrometheusRuleRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       monitoringv1.PrometheusRuleSpec

	Instance *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newPrometheusRule returns a new PrometheusRule instance for the given ArgoCD.
func newPrometheusRule(objectMeta metav1.ObjectMeta, spec monitoringv1.PrometheusRuleSpec) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func RequestPrometheusRule(request PrometheusRuleRequest) (*monitoringv1.PrometheusRule, error) {
	var (
		mutationErr error
	)
	prometheusRule := newPrometheusRule(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, prometheusRule, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return prometheusRule, fmt.Errorf("RequestPrometheusRule: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return prometheusRule, nil
}

// CreatePrometheusRule creates the specified PrometheusRule using the provided client.
func CreatePrometheusRule(prometheusRule *monitoringv1.PrometheusRule, client cntrlClient.Client) error {
	return resource.CreateObject(prometheusRule, client)
}

// UpdatePrometheusRule updates the specified PrometheusRule using the provided client.
func UpdatePrometheusRule(prometheusRule *monitoringv1.PrometheusRule, client cntrlClient.Client) error {
	return resource.UpdateObject(prometheusRule, client)
}

// DeletePrometheusRule deletes the PrometheusRule with the given name and namespace using the provided client.
func DeletePrometheusRule(name, namespace string, client cntrlClient.Client) error {
	prometheusRule := &monitoringv1.PrometheusRule{}
	return resource.DeleteObject(name, namespace, prometheusRule, client)
}

// GetPrometheusRule retrieves the PrometheusRule with the given name and namespace using the provided client.
func GetPrometheusRule(name, namespace string, client cntrlClient.Client) (*monitoringv1.PrometheusRule, error) {
	prometheusRule := &monitoringv1.PrometheusRule{}
	obj, err := resource.GetObject(name, namespace, prometheusRule, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a monitoringv1.PrometheusRule
	prometheusRule, ok := obj.(*monitoringv1.PrometheusRule)
	if !ok {
		return nil, errors.New("failed to assert the object as a monitoringv1.PrometheusRule")
	}
	return prometheusRule, nil
}

// ListPrometheusRules returns a list of PrometheusRule objects in the specified namespace using the provided client and list options.
func ListPrometheusRules(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*monitoringv1.PrometheusRuleList, error) {
	prometheusRuleList := &monitoringv1.PrometheusRuleList{}
	obj, err := resource.ListObjects(namespace, prometheusRuleList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a monitoringv1.PrometheusRuleList
	prometheusRuleList, ok := obj.(*monitoringv1.PrometheusRuleList)
	if !ok {
		return nil, errors.New("failed to assert the object as a monitoringv1.PrometheusRuleList")
	}
	return prometheusRuleList, nil
}
