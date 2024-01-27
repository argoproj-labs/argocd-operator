package networking

import (
	"context"
	"sort"
	"testing"

	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

type ingressOpt func(*networkingv1.Ingress)

func getTestIngress(opts ...ingressOpt) *networkingv1.Ingress {
	nginx := "nginx"
	desiredIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      test.TestName,
			Namespace: test.TestNamespace,
			Labels: map[string]string{
				common.AppK8sKeyName:      test.TestInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
				common.AppK8sKeyComponent: test.TestComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      test.TestInstance,
				common.ArgoCDArgoprojKeyNamespace: test.TestInstanceNamespace,
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &nginx,
			Rules: []networkingv1.IngressRule{
				{
					Host: "foo.bar.com",
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{
						"test.host.com",
					},
					SecretName: common.ArgoCDSecretName,
				},
			},
		},
	}

	for _, opt := range opts {
		opt(desiredIngress)
	}
	return desiredIngress
}

func TestRequestIngress(t *testing.T) {

	s := scheme.Scheme
	assert.NoError(t, networkingv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()
	nginx := "nginx"

	tests := []struct {
		name           string
		ingressReq     IngressRequest
		desiredIngress *networkingv1.Ingress
		mutation       bool
		wantErr        bool
	}{
		{
			name: "request ingress, no mutation",
			ingressReq: IngressRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.TestName,
					Namespace: test.TestNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      test.TestInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: test.TestComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      test.TestInstance,
						common.ArgoCDArgoprojKeyNamespace: test.TestInstanceNamespace,
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &nginx,
					Rules: []networkingv1.IngressRule{
						{
							Host: "foo.bar.com",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{
								"test.host.com",
							},
							SecretName: common.ArgoCDSecretName,
						},
					},
				},
			},
			mutation:       false,
			desiredIngress: getTestIngress(func(i *networkingv1.Ingress) {}),
			wantErr:        false,
		},
		{
			name: "request ingress, no mutation, custom name, labels, annotations",
			ingressReq: IngressRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.TestName,
					Namespace: test.TestNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      test.TestInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: test.TestComponent,
						test.TestKey:              test.TestVal,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      test.TestInstance,
						common.ArgoCDArgoprojKeyNamespace: test.TestInstanceNamespace,
						test.TestKey:                      test.TestVal,
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &nginx,
					Rules: []networkingv1.IngressRule{
						{
							Host: "foo.bar.com",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{
								"test.host.com",
							},
							SecretName: common.ArgoCDSecretName,
						},
					},
				},
			},
			mutation: false,
			desiredIngress: getTestIngress(func(i *networkingv1.Ingress) {
				i.Name = test.TestName
				i.Labels = util.MergeMaps(i.Labels, test.TestKVP)
				i.Annotations = util.MergeMaps(i.Annotations, test.TestKVP)
			}),
			wantErr: false,
		},
		{
			name: "request ingress, successful mutation",
			ingressReq: IngressRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testIngressNameMutated,
					Namespace: test.TestNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      test.TestInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: test.TestComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      test.TestInstance,
						common.ArgoCDArgoprojKeyNamespace: test.TestInstanceNamespace,
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &nginx,
					Rules: []networkingv1.IngressRule{
						{
							Host: "foo.bar.com",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{
								"test.host.com",
							},
							SecretName: common.ArgoCDSecretName,
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:       true,
			desiredIngress: getTestIngress(func(i *networkingv1.Ingress) { i.Name = testIngressNameMutated }),
			wantErr:        false,
		},
		{
			name: "request ingress, failed mutation",
			ingressReq: IngressRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.TestName,
					Namespace: test.TestNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      test.TestInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: test.TestComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      test.TestInstance,
						common.ArgoCDArgoprojKeyNamespace: test.TestInstanceNamespace,
					},
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: &nginx,
					Rules: []networkingv1.IngressRule{
						{
							Host: "foo.bar.com",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{
								"test.host.com",
							},
							SecretName: common.ArgoCDSecretName,
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:       true,
			desiredIngress: getTestIngress(func(i *networkingv1.Ingress) {}),
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotIngress, err := RequestIngress(test.ingressReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredIngress, gotIngress)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateIngress(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, networkingv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	desiredIngress := getTestIngress(func(i *networkingv1.Ingress) {
		i.TypeMeta = metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking.k8s.io/v1",
		}
		i.Name = test.TestName
		i.Namespace = test.TestNamespace
	})
	err := CreateIngress(desiredIngress, testClient)
	assert.NoError(t, err)

	createdIngress := &networkingv1.Ingress{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdIngress)

	assert.NoError(t, err)
	assert.Equal(t, desiredIngress, createdIngress)
}

func TestGetIngress(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, networkingv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = test.TestName
		i.Namespace = test.TestNamespace
	})).Build()

	_, err := GetIngress(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetIngress(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListIngresss(t *testing.T) {
	ingress1 := getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = "ingress-1"
		i.Namespace = test.TestNamespace
		i.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	ingress2 := getTestIngress(func(i *networkingv1.Ingress) { i.Name = "ingress-2" })
	ingress3 := getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = "ingress-3"
		i.Namespace = test.TestNamespace
		i.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, networkingv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		ingress1, ingress2, ingress3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredIngresss := []string{"ingress-1", "ingress-3"}

	existingIngressList, err := ListIngresss(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingIngresss := []string{}
	for _, ingress := range existingIngressList.Items {
		existingIngresss = append(existingIngresss, ingress.Name)
	}
	sort.Strings(existingIngresss)

	assert.Equal(t, desiredIngresss, existingIngresss)
}

func TestUpdateIngress(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, networkingv1.AddToScheme(s))

	// Create the initial Ingress
	initialIngress := getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = test.TestName
		i.Namespace = test.TestNamespace
	})

	// Create the client with the initial Ingress
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialIngress).Build()

	// Fetch the Ingress from the client
	desiredIngress := &networkingv1.Ingress{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}, desiredIngress)
	assert.NoError(t, err)

	desiredIngress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: "argocd",
		},
	}

	err = UpdateIngress(desiredIngress, testClient)
	assert.NoError(t, err)

	existingIngress := &networkingv1.Ingress{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingIngress)

	assert.NoError(t, err)
	assert.Equal(t, desiredIngress.Spec.Rules, existingIngress.Spec.Rules)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingIngress = getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = test.TestName
		i.Labels = nil
	})
	err = UpdateIngress(existingIngress, testClient)
	assert.Error(t, err)
}

func TestDeleteIngress(t *testing.T) {
	testIngress := getTestIngress(func(i *networkingv1.Ingress) {
		i.Name = test.TestName
		i.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testIngress).Build()

	err := DeleteIngress(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingIngress := &networkingv1.Ingress{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingIngress)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
