package monitoring

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type prometheusRuleOpt func(*monitoringv1.PrometheusRule)

func getTestPrometheusRule(opts ...prometheusRuleOpt) *monitoringv1.PrometheusRule {
	desiredPrometheusRule := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.AppK8sKeyName:      testInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
				common.AppK8sKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      testInstance,
				common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
			},
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name: common.ArgoCDComponentStatus,
					Rules: []monitoringv1.Rule{
						{
							Alert: "test alert",
						},
					},
				},
			},
		},
	}

	for _, opt := range opts {
		opt(desiredPrometheusRule)
	}
	return desiredPrometheusRule
}

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
			name: "request prometheusRule, no mutation",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name: common.ArgoCDComponentStatus,
							Rules: []monitoringv1.Rule{
								{
									Alert: "test alert",
								},
							},
						},
					},
				},
			},
			mutation:              false,
			desiredPrometheusRule: getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {}),
			wantErr:               false,
		},
		{
			name: "request prometheusRule, no mutation, custom name, labels, annotations",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
						testKey:                   testVal,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
						testKey:                           testVal,
					},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name: common.ArgoCDComponentStatus,
							Rules: []monitoringv1.Rule{
								{
									Alert: "test alert",
								},
							},
						},
					},
				},
			},
			mutation: false,
			desiredPrometheusRule: getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
				pr.Name = testName
				pr.Labels = util.MergeMaps(pr.Labels, testKVP)
				pr.Annotations = util.MergeMaps(pr.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request prometheusRule, successful mutation",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testPrometheusRuleNameMutated,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name: common.ArgoCDComponentStatus,
							Rules: []monitoringv1.Rule{
								{
									Alert: "test alert",
								},
							},
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:              true,
			desiredPrometheusRule: getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) { pr.Name = testPrometheusRuleNameMutated }),
			wantErr:               false,
		},
		{
			name: "request prometheusRule, failed mutation",
			prometheusRuleReq: PrometheusRuleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{
						{
							Name: common.ArgoCDComponentStatus,
							Rules: []monitoringv1.Rule{
								{
									Alert: "test alert",
								},
							},
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:              true,
			desiredPrometheusRule: getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {}),
			wantErr:               true,
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

	desiredPrometheusRule := getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.TypeMeta = metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		}
		pr.Name = testName
		pr.Namespace = testNamespace
	})
	err := CreatePrometheusRule(desiredPrometheusRule, testClient)
	assert.NoError(t, err)

	createdPrometheusRule := &monitoringv1.PrometheusRule{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdPrometheusRule)

	assert.NoError(t, err)
	assert.Equal(t, desiredPrometheusRule, createdPrometheusRule)
}

func TestGetPrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = testName
		pr.Namespace = testNamespace
	})).Build()

	_, err := GetPrometheusRule(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetPrometheusRule(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListPrometheusRules(t *testing.T) {
	prometheusRule1 := getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = "prometheusRule-1"
		pr.Namespace = testNamespace
		pr.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	prometheusRule2 := getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) { pr.Name = "prometheusRule-2" })
	prometheusRule3 := getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = "prometheusRule-3"
		pr.Namespace = testNamespace
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

	existingPrometheusRuleList, err := ListPrometheusRules(testNamespace, testClient, listOpts)
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
	initialPrometheusRule := getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = testName
		pr.Namespace = testNamespace
	})

	// Create the client with the initial PrometheusRule
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialPrometheusRule).Build()

	// Fetch the PrometheusRule from the client
	desiredPrometheusRule := &monitoringv1.PrometheusRule{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: testName, Namespace: testNamespace}, desiredPrometheusRule)
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
		Namespace: testNamespace,
		Name:      testName,
	}, existingPrometheusRule)

	assert.NoError(t, err)
	assert.Equal(t, desiredPrometheusRule.Spec.Groups, existingPrometheusRule.Spec.Groups)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingPrometheusRule = getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = testName
		pr.Labels = nil
	})
	err = UpdatePrometheusRule(existingPrometheusRule, testClient)
	assert.Error(t, err)
}

func TestDeletePrometheusRule(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestPrometheusRule(func(pr *monitoringv1.PrometheusRule) {
		pr.Name = testName
		pr.Namespace = testNamespace
	})).Build()

	err := DeletePrometheusRule(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingPrometheusRule := &monitoringv1.PrometheusRule{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingPrometheusRule)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	err = DeletePrometheusRule(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
