package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestStatefulSet(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name               string
		statefultSetReq    StatefulSetRequest
		desiredStatefulSet *appsv1.StatefulSet
		wantErr            bool
	}{
		{
			name: "request StatefulSet",
			statefultSetReq: StatefulSetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
			},
			desiredStatefulSet: test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
				ss.Name = test.TestName
				ss.Namespace = test.TestNamespace
				ss.Labels = test.TestKVP
				ss.Annotations = test.TestKVP
				ss.Spec.Selector.MatchLabels = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request StatefulSet, successful mutation",
			statefultSetReq: StatefulSetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredStatefulSet: test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
				ss.Name = test.TestNameMutated
				ss.Namespace = test.TestNamespace
				ss.Labels = test.TestKVP
				ss.Annotations = test.TestKVP
				ss.Spec.Replicas = &testReplicasMutated
				ss.Spec.Selector.MatchLabels = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request StatefulSet, failed mutation",
			statefultSetReq: StatefulSetRequest{
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
			desiredStatefulSet: test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
				ss.Name = test.TestNameMutated
				ss.Namespace = test.TestNamespace
				ss.Labels = test.TestKVP
				ss.Annotations = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotStatefulSet, err := RequestStatefulSet(test.statefultSetReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredStatefulSet, gotStatefulSet)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateStatefulSet(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredStatefulSet := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.TypeMeta = metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		}
		ss.Name = test.TestName
		ss.Namespace = test.TestNamespace
		ss.Labels = test.TestKVP
		ss.Annotations = test.TestKVP
	})
	err := CreateStatefulSet(desiredStatefulSet, testClient)
	assert.NoError(t, err)

	createdStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdStatefulSet)

	assert.NoError(t, err)
	assert.Equal(t, desiredStatefulSet, createdStatefulSet)
}

func TestGetStatefulSet(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = test.TestName
		ss.Namespace = test.TestNamespace

	})).Build()

	_, err := GetStatefulSet(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetStatefulSet(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListStatefulSets(t *testing.T) {
	StatefulSet1 := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-1"
		ss.Namespace = test.TestNamespace
		ss.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	StatefulSet2 := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-2"
		ss.Namespace = test.TestNamespace
	})
	StatefulSet3 := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-3"
		ss.Labels[common.AppK8sKeyComponent] = "new-component-2"
		ss.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		StatefulSet1, StatefulSet2, StatefulSet3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredStatefulSets := []string{"StatefulSet-1", "StatefulSet-3"}

	existingStatefulSetList, err := ListStatefulSets(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingStatefulSets := []string{}
	for _, StatefulSet := range existingStatefulSetList.Items {
		existingStatefulSets = append(existingStatefulSets, StatefulSet.Name)
	}
	sort.Strings(existingStatefulSets)

	assert.Equal(t, desiredStatefulSets, existingStatefulSets)
}

func TestUpdateStatefulSet(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = test.TestName
		ss.Namespace = test.TestNamespace
	})).Build()

	desiredStatefulSet := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = test.TestName
		ss.Namespace = test.TestNamespace
		ss.Spec.Template.Spec.NodeSelector = map[string]string{
			"kubernetes.io/os": "linux",
		}

	})
	err := UpdateStatefulSet(desiredStatefulSet, testClient)
	assert.NoError(t, err)

	existingStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingStatefulSet)

	assert.NoError(t, err)
	assert.Equal(t, desiredStatefulSet.Spec, existingStatefulSet.Spec)

	testClient = fake.NewClientBuilder().Build()
	existingStatefulSet = test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = test.TestName
	})
	err = UpdateStatefulSet(existingStatefulSet, testClient)
	assert.Error(t, err)
}

func TestDeleteStatefulSet(t *testing.T) {
	testss := test.MakeTestStatefulSet(nil, func(ss *appsv1.StatefulSet) {
		ss.Name = test.TestName
		ss.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testss).Build()

	err := DeleteStatefulSet(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingStatefulSet)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
