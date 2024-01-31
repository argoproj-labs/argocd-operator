package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	autoscaling "k8s.io/api/autoscaling/v1"
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

func TestRequestHorizontalPodAutoscaler(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name                           string
		hpaReq                         HorizontalPodAutoscalerRequest
		desiredHorizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler
		wantErr                        bool
	}{
		{
			name: "request horizontalPodAutoscaler",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
			},
			desiredHorizontalPodAutoscaler: test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = test.TestName
				hpa.Namespace = test.TestNamespace
				hpa.Labels = test.TestKVP
				hpa.Annotations = test.TestKVP
				hpa.Spec.MaxReplicas = testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request horizontalPodAutoscaler, successful mutation",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredHorizontalPodAutoscaler: test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = test.TestNameMutated
				hpa.Namespace = test.TestNamespace
				hpa.Labels = test.TestKVP
				hpa.Annotations = test.TestKVP
				hpa.Spec.MaxReplicas = testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request horizontalPodAutoscaler, failed mutation",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredHorizontalPodAutoscaler: test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = test.TestName
				hpa.Namespace = test.TestNamespace
				hpa.Labels = test.TestKVP
				hpa.Annotations = test.TestKVP
				hpa.Spec.MaxReplicas = testReplicasMutated
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotHorizontalPodAutoscaler, err := RequestHorizontalPodAutoscaler(test.hpaReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredHorizontalPodAutoscaler, gotHorizontalPodAutoscaler)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateHorizontalPodAutoscaler(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredHorizontalPodAutoscaler := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.TypeMeta = metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		}
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
		hpa.Labels = test.TestKVP
		hpa.Annotations = test.TestKVP
	})
	err := CreateHorizontalPodAutoscaler(desiredHorizontalPodAutoscaler, testClient)
	assert.NoError(t, err)

	createdHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdHorizontalPodAutoscaler)

	assert.NoError(t, err)
	assert.Equal(t, desiredHorizontalPodAutoscaler, createdHorizontalPodAutoscaler)
}

func TestGetHorizontalPodAutoscaler(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
	})).Build()

	_, err := GetHorizontalPodAutoscaler(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetHorizontalPodAutoscaler(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListHorizontalPodAutoscalers(t *testing.T) {
	horizontalPodAutoscaler1 := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = "horizontalPodAutoscaler-1"
		hpa.Namespace = test.TestNamespace
		hpa.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	horizontalPodAutoscaler2 := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = "horizontalPodAutoscaler-2"
		hpa.Namespace = test.TestNamespace
	})
	horizontalPodAutoscaler3 := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = "horizontalPodAutoscaler-3"
		hpa.Labels[common.AppK8sKeyComponent] = "new-component-2"
		hpa.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		horizontalPodAutoscaler1, horizontalPodAutoscaler2, horizontalPodAutoscaler3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredHorizontalPodAutoscalers := []string{"horizontalPodAutoscaler-1", "horizontalPodAutoscaler-3"}

	existingHorizontalPodAutoscalerList, err := ListHorizontalPodAutoscalers(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingHorizontalPodAutoscalers := []string{}
	for _, horizontalPodAutoscaler := range existingHorizontalPodAutoscalerList.Items {
		existingHorizontalPodAutoscalers = append(existingHorizontalPodAutoscalers, horizontalPodAutoscaler.Name)
	}
	sort.Strings(existingHorizontalPodAutoscalers)

	assert.Equal(t, desiredHorizontalPodAutoscalers, existingHorizontalPodAutoscalers)
}

func TestUpdateHorizontalPodAutoscaler(t *testing.T) {
	var (
		maxReplicas int32 = 3
		minReplicas int32 = 1
	)
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
	})).Build()

	desiredHorizontalPodAutoscaler := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas
	})
	err := UpdateHorizontalPodAutoscaler(desiredHorizontalPodAutoscaler, testClient)
	assert.NoError(t, err)

	existingHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingHorizontalPodAutoscaler)

	assert.NoError(t, err)
	assert.Equal(t, desiredHorizontalPodAutoscaler.Spec, existingHorizontalPodAutoscaler.Spec)

	testClient = fake.NewClientBuilder().Build()
	existingHorizontalPodAutoscaler = test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
	})
	err = UpdateHorizontalPodAutoscaler(existingHorizontalPodAutoscaler, testClient)
	assert.Error(t, err)
}

func TestDeleteHorizontalPodAutoscaler(t *testing.T) {
	testHPA := test.MakeTestHPA(nil, func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = test.TestName
		hpa.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testHPA).Build()

	err := DeleteHorizontalPodAutoscaler(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingHPA := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingHPA)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
