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
)

type statefulSetOpt func(*appsv1.StatefulSet)

func getTestStatefulSet(opts ...statefulSetOpt) *appsv1.StatefulSet {
	desiredStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{},
		},
	}

	for _, opt := range opts {
		opt(desiredStatefulSet)
	}
	return desiredStatefulSet
}

func TestRequestStatefulSet(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name               string
		statefultSetReq    StatefulSetRequest
		desiredStatefulSet *appsv1.StatefulSet
		wantErr            bool
	}{
		{
			name: "request StatefulSet, no mutation, custom name, labels",
			statefultSetReq: StatefulSetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: testKVP,
					},
				},
			},
			desiredStatefulSet: getTestStatefulSet(func(ss *appsv1.StatefulSet) {
				ss.Name = testName
				ss.Namespace = testNamespace
				ss.Labels = testKVP
				ss.Annotations = testKVP
				ss.Spec.Selector.MatchLabels = testKVP
			}),
			wantErr: false,
		},
		{
			name: "request StatefulSet, successful mutation",
			statefultSetReq: StatefulSetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: testKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredStatefulSet: getTestStatefulSet(func(ss *appsv1.StatefulSet) {
				ss.Name = testNameMutated
				ss.Namespace = testNamespace
				ss.Labels = testKVP
				ss.Annotations = testKVP
				ss.Spec.Replicas = &testReplicasMutated
				ss.Spec.Selector.MatchLabels = testKVP
			}),
			wantErr: false,
		},
		{
			name: "request StatefulSet, failed mutation",
			statefultSetReq: StatefulSetRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredStatefulSet: getTestStatefulSet(func(ss *appsv1.StatefulSet) {
				ss.Name = testNameMutated
				ss.Namespace = testNamespace
				ss.Labels = testKVP
				ss.Annotations = testKVP
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

	desiredStatefulSet := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.TypeMeta = metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		}
		ss.Name = testName
		ss.Namespace = testNamespace
		ss.Labels = testKVP
		ss.Annotations = testKVP
	})
	err := CreateStatefulSet(desiredStatefulSet, testClient)
	assert.NoError(t, err)

	createdStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdStatefulSet)

	assert.NoError(t, err)
	assert.Equal(t, desiredStatefulSet, createdStatefulSet)
}

func TestGetStatefulSet(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = testName
		ss.Namespace = testNamespace

	})).Build()

	_, err := GetStatefulSet(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetStatefulSet(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListStatefulSets(t *testing.T) {
	StatefulSet1 := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-1"
		ss.Namespace = testNamespace
		ss.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	StatefulSet2 := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-2"
		ss.Namespace = testNamespace
	})
	StatefulSet3 := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = "StatefulSet-3"
		ss.Labels[common.AppK8sKeyComponent] = "new-component-2"
		ss.Namespace = testNamespace
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

	existingStatefulSetList, err := ListStatefulSets(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingStatefulSets := []string{}
	for _, StatefulSet := range existingStatefulSetList.Items {
		existingStatefulSets = append(existingStatefulSets, StatefulSet.Name)
	}
	sort.Strings(existingStatefulSets)

	assert.Equal(t, desiredStatefulSets, existingStatefulSets)
}

func TestUpdateStatefulSet(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = testName
		ss.Namespace = testNamespace
	})).Build()

	desiredStatefulSet := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = testName
		ss.Namespace = testNamespace
		ss.Spec.Template.Spec.NodeSelector = map[string]string{
			"kubernetes.io/os": "linux",
		}

	})
	err := UpdateStatefulSet(desiredStatefulSet, testClient)
	assert.NoError(t, err)

	existingStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingStatefulSet)

	assert.NoError(t, err)
	assert.Equal(t, desiredStatefulSet.Spec, existingStatefulSet.Spec)

	testClient = fake.NewClientBuilder().Build()
	existingStatefulSet = getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = testName
	})
	err = UpdateStatefulSet(existingStatefulSet, testClient)
	assert.Error(t, err)
}

func TestDeleteStatefulSet(t *testing.T) {
	testss := getTestStatefulSet(func(ss *appsv1.StatefulSet) {
		ss.Name = testName
		ss.Namespace = testNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testss).Build()

	err := DeleteStatefulSet(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingStatefulSet := &appsv1.StatefulSet{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingStatefulSet)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
