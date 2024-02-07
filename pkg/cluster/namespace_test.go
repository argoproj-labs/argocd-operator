package cluster

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *corev1.Namespace:
		if _, ok := obj.Labels[test.TestKey]; ok {
			obj.Labels[test.TestKey] = test.TestValMutated
			return nil
		}
	}
	return errors.New("test-mutation-error")
}

func TestRequestNamespace(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name             string
		nsReq            NamespaceRequest
		desiredNamespace *corev1.Namespace
		wantErr          bool
	}{
		{
			name: "request namespace",
			nsReq: NamespaceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:   test.TestName,
					Labels: test.TestKVP,
				},
			},
			desiredNamespace: test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
				ns.Name = test.TestName
				ns.Labels = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request namespace, successful mutation",
			nsReq: NamespaceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:   test.TestName,
					Labels: test.TestKVP,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredNamespace: test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
				ns.Name = test.TestName
				ns.Labels = test.TestKVPMutated
			}),
			wantErr: false,
		},
		{
			name: "request namespace, failed mutation",
			nsReq: NamespaceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:   test.TestName,
					Labels: test.TestKVP,
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredNamespace: test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
				ns.Name = test.TestName
				ns.Labels = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotNamespace, err := RequestNamespace(test.nsReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredNamespace, gotNamespace)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCreateNamespace(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredNamespace := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
		ns.TypeMeta = metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		}
		ns.Labels = test.TestKVP
	})
	err := CreateNamespace(desiredNamespace, testClient)
	assert.NoError(t, err)

	createdNamespace := &corev1.Namespace{}
	err = testClient.Get(context.TODO(), cntrlClient.ObjectKey{Name: test.TestName}, createdNamespace)

	assert.NoError(t, err)
	assert.Equal(t, desiredNamespace, createdNamespace)
}

func TestGetNamespace(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
	})).Build()

	_, err := GetNamespace(test.TestName, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetNamespace(test.TestName, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListNamespaces(t *testing.T) {
	namespace1 := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = "namespace-1"
		ns.Labels = map[string]string{
			common.AppK8sKeyComponent: "new-component-1",
		}
	})
	namespace2 := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) { ns.Name = "namespace-2" })
	namespace3 := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = "namespace-3"
		ns.Labels = map[string]string{
			common.AppK8sKeyComponent: "new-component-2",
		}
	})

	testClient := fake.NewClientBuilder().WithObjects(
		namespace1, namespace2, namespace3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredNamespaces := []string{"namespace-1", "namespace-3"}

	existingNamespaceList, err := ListNamespaces(testClient, listOpts)
	assert.NoError(t, err)

	existingNamespaces := []string{}
	for _, ns := range existingNamespaceList.Items {
		existingNamespaces = append(existingNamespaces, ns.Name)
	}
	sort.Strings(existingNamespaces)

	assert.Equal(t, desiredNamespaces, existingNamespaces)
}

func TestUpdateNamespace(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
	})).Build()

	desiredNamespace := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
		ns.Labels = test.TestKVP
	})
	err := UpdateNamespace(desiredNamespace, testClient)
	assert.NoError(t, err)

	existingNamespace := &corev1.Namespace{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingNamespace)

	assert.NoError(t, err)
	assert.Equal(t, desiredNamespace.Labels, existingNamespace.Labels)

	testClient = fake.NewClientBuilder().Build()
	existingNamespace = test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
		ns.Labels = test.TestKVP
	})
	err = UpdateNamespace(existingNamespace, testClient)
	assert.Error(t, err)

}

func TestDeleteNamespace(t *testing.T) {
	testNamespace := test.MakeTestNamespace(nil, func(ns *corev1.Namespace) {
		ns.Name = test.TestName
	})

	testClient := fake.NewClientBuilder().WithObjects(testNamespace).Build()

	err := DeleteNamespace(test.TestName, testClient)
	assert.NoError(t, err)

	existingNamespace := &corev1.Namespace{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingNamespace)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
