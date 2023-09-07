package monitoring

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// PrometheusRuleRequest objects contain all the required information to produce a prometheusRule object in return
type PrometheusRuleRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       monitoringv1.PrometheusRuleSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    ctrlClient.Client
}

// newPrometheusRule returns a new PrometheusRule instance for the given ArgoCD.
func newPrometheusRule(objectMeta metav1.ObjectMeta, spec monitoringv1.PrometheusRuleSpec) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func CreatePrometheusRule(prometheusRule *monitoringv1.PrometheusRule, client ctrlClient.Client) error {
	return client.Create(context.TODO(), prometheusRule)
}

// UpdatePrometheusRule updates the specified PrometheusRule using the provided client.
func UpdatePrometheusRule(prometheusRule *monitoringv1.PrometheusRule, client ctrlClient.Client) error {
	_, err := GetPrometheusRule(prometheusRule.Name, prometheusRule.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), prometheusRule); err != nil {
		return err
	}
	return nil
}

func DeletePrometheusRule(name, namespace string, client ctrlClient.Client) error {
	existingPrometheusRule, err := GetPrometheusRule(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingPrometheusRule); err != nil {
		return err
	}
	return nil
}

func GetPrometheusRule(name, namespace string, client ctrlClient.Client) (*monitoringv1.PrometheusRule, error) {
	existingPrometheusRule := &monitoringv1.PrometheusRule{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingPrometheusRule)
	if err != nil {
		return nil, err
	}
	return existingPrometheusRule, nil
}

func ListPrometheusRules(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*monitoringv1.PrometheusRuleList, error) {
	existingPrometheusRules := &monitoringv1.PrometheusRuleList{}
	err := client.List(context.TODO(), existingPrometheusRules, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingPrometheusRules, nil
}

func RequestPrometheusRule(request PrometheusRuleRequest) (*monitoringv1.PrometheusRule, error) {
	var (
		mutationErr error
	)
	prometheusRule := newPrometheusRule(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, prometheusRule, request.Client)
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
