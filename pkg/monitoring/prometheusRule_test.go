package monitoring

import (
	"context"
	"sort"
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestPrometheusRule(t *testing.T) {

	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	tests := []struct {
		name                  string
		prometheusRuleReq     PrometheusRuleRequest
		desiredPrometheusRule *monitoringv1.PrometheusRule
		mutation              bool
		wantErr               bool
	}{
		{
			name: "request prometheusRule",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
			},
			mutation: false,
			desiredPrometheusRule: test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
				pr.Labels = test.TestKVP
				pr.Annotations = test.TestKVP

			}),
			wantErr: false,
		},
		{
			name: "request prometheusRule, successful mutation",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},

				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation: true,
			desiredPrometheusRule: test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
				pr.Name = test.TestNameMutated
				pr.Labels = test.TestKVP
				pr.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request prometheusRule, failed mutation",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},

				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation: true,
			desiredPrometheusRule: test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
				pr.Labels = test.TestKVP
				pr.Annotations = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotPrometheusRule, err := RequestPrometheusRule(test.prometheusRuleReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredPrometheusRule, gotPrometheusRule)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreatePrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	desiredPrometheusRule := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.TypeMeta = metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		}
		pr.Name = test.TestName
		pr.Namespace = test.TestNamespace
		pr.Labels = test.TestKVP
		pr.Annotations = test.TestKVP
	})
	err := CreatePrometheusRule(desiredPrometheusRule, testClient)
	assert.NoError(t, err)

	createdPrometheusRule := &monitoringv1.PrometheusRule{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdPrometheusRule)

	assert.NoError(t, err)
	assert.Equal(t, desiredPrometheusRule, createdPrometheusRule)
}

func TestGetPrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = test.TestName
		pr.Namespace = test.TestNamespace
	})).Build()

	_, err := GetPrometheusRule(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetPrometheusRule(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListPrometheusRules(t *testing.T) {
	prometheusRule1 := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = "prometheusRule-1"
		pr.Namespace = test.TestNamespace
		pr.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	prometheusRule2 := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) { pr.Name = "prometheusRule-2" })
	prometheusRule3 := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = "prometheusRule-3"
		pr.Namespace = test.TestNamespace
		pr.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		prometheusRule1, prometheusRule2, prometheusRule3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredPrometheusRules := []string{"prometheusRule-1", "prometheusRule-3"}

	existingPrometheusRuleList, err := ListPrometheusRules(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingPrometheusRules := []string{}
	for _, prometheusRule := range existingPrometheusRuleList.Items {
		existingPrometheusRules = append(existingPrometheusRules, prometheusRule.Name)
	}
	sort.Strings(existingPrometheusRules)

	assert.Equal(t, desiredPrometheusRules, existingPrometheusRules)
}

func TestUpdatePrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	// Create the initial PrometheusRule
	initialPrometheusRule := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = test.TestName
		pr.Namespace = test.TestNamespace
	})

	// Create the client with the initial PrometheusRule
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialPrometheusRule).Build()

	// Fetch the PrometheusRule from the client
	desiredPrometheusRule := &monitoringv1.PrometheusRule{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}, desiredPrometheusRule)
	assert.NoError(t, err)

	newRuleGroups := []monitoringv1.RuleGroup{
		{
			Name: common.ArgoCDComponentStatus,
			Rules: []monitoringv1.Rule{
				{
					Alert: "ApplicationControllerNotReady",
					Annotations: map[string]string{
						"message": "test message",
					},
					Expr: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "test-expr",
					},
					For: "1m",
					Labels: map[string]string{
						"severity": "critical",
					},
				},
			},
		}}

	desiredPrometheusRule.Spec.Groups = newRuleGroups

	err = UpdatePrometheusRule(desiredPrometheusRule, testClient)
	assert.NoError(t, err)

	existingPrometheusRule := &monitoringv1.PrometheusRule{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingPrometheusRule)

	assert.NoError(t, err)
	assert.Equal(t, desiredPrometheusRule.Spec.Groups, existingPrometheusRule.Spec.Groups)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingPrometheusRule = test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = test.TestName
		pr.Labels = nil
	})
	err = UpdatePrometheusRule(existingPrometheusRule, testClient)
	assert.Error(t, err)
}

func TestDeletePrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testPrometheusRule := test.MakeTestPrometheusRule(nil, func(pr *monitoringv1.PrometheusRule) {
		pr.Name = test.TestName
		pr.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(testPrometheusRule).Build()

	err := DeletePrometheusRule(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingPrometheusRule := &monitoringv1.PrometheusRule{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingPrometheusRule)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
