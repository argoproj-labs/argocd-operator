package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/stretchr/testify/assert"
	autoscaling "k8s.io/api/autoscaling/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type horizontalPodAutoscalerOpt func(*autoscaling.HorizontalPodAutoscaler)

func getTestHorizontalPodAutoscaler(opts ...horizontalPodAutoscalerOpt) *autoscaling.HorizontalPodAutoscaler {
	desiredHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{},
	}

	for _, opt := range opts {
		opt(desiredHorizontalPodAutoscaler)
	}
	return desiredHorizontalPodAutoscaler
}

func TestRequestHorizontalPodAutoscaler(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name                           string
		hpaReq                         HorizontalPodAutoscalerRequest
		desiredHorizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler
		wantErr                        bool
	}{
		{
			name: "request horizontalPodAutoscaler, no mutation, custom name, labels, annotations",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
			},
			desiredHorizontalPodAutoscaler: getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = testName
				hpa.Namespace = testNamespace
				hpa.Labels = testKVP
				hpa.Annotations = testKVP
				hpa.Spec.MaxReplicas = testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request horizontalPodAutoscaler, successful mutation",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredHorizontalPodAutoscaler: getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = testNameMutated
				hpa.Namespace = testNamespace
				hpa.Labels = testKVP
				hpa.Annotations = testKVP
				hpa.Spec.MaxReplicas = testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request horizontalPodAutoscaler, failed mutation",
			hpaReq: HorizontalPodAutoscalerRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: autoscaling.HorizontalPodAutoscalerSpec{
					MaxReplicas: testReplicasMutated,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredHorizontalPodAutoscaler: getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
				hpa.Name = testName
				hpa.Namespace = testNamespace
				hpa.Labels = testKVP
				hpa.Annotations = testKVP
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

	desiredHorizontalPodAutoscaler := getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.TypeMeta = metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: "autoscaling/v1",
		}
		hpa.Name = testName
		hpa.Namespace = testNamespace
		hpa.Labels = testKVP
		hpa.Annotations = testKVP
	})
	err := CreateHorizontalPodAutoscaler(desiredHorizontalPodAutoscaler, testClient)
	assert.NoError(t, err)

	createdHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdHorizontalPodAutoscaler)

	assert.NoError(t, err)
	assert.Equal(t, desiredHorizontalPodAutoscaler, createdHorizontalPodAutoscaler)
}

func TestGetHorizontalPodAutoscaler(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = testName
		hpa.Namespace = testNamespace
	})).Build()

	_, err := GetHorizontalPodAutoscaler(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetHorizontalPodAutoscaler(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListHorizontalPodAutoscalers(t *testing.T) {
	horizontalPodAutoscaler1 := getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = "horizontalPodAutoscaler-1"
		hpa.Namespace = testNamespace
		hpa.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	horizontalPodAutoscaler2 := getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) { hpa.Name = "horizontalPodAutoscaler-2" })
	horizontalPodAutoscaler3 := getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = "horizontalPodAutoscaler-3"
		hpa.Labels[common.AppK8sKeyComponent] = "new-component-2"
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

	existingHorizontalPodAutoscalerList, err := ListHorizontalPodAutoscalers(testNamespace, testClient, listOpts)
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
	testClient := fake.NewClientBuilder().WithObjects(getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = testName
		hpa.Namespace = testNamespace
	})).Build()

	desiredHorizontalPodAutoscaler := getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = testName
		hpa.Namespace = testNamespace
		hpa.Spec.MinReplicas = &minReplicas
		hpa.Spec.MaxReplicas = maxReplicas
	})
	err := UpdateHorizontalPodAutoscaler(desiredHorizontalPodAutoscaler, testClient)
	assert.NoError(t, err)

	existingHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingHorizontalPodAutoscaler)

	assert.NoError(t, err)
	assert.Equal(t, desiredHorizontalPodAutoscaler.Spec, existingHorizontalPodAutoscaler.Spec)

	testClient = fake.NewClientBuilder().Build()
	existingHorizontalPodAutoscaler = getTestHorizontalPodAutoscaler(func(hpa *autoscaling.HorizontalPodAutoscaler) {
		hpa.Name = testName
		hpa.Namespace = testNamespace
	})
	err = UpdateHorizontalPodAutoscaler(existingHorizontalPodAutoscaler, testClient)
	assert.Error(t, err)
}

func TestDeleteHorizontalPodAutoscaler(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestHorizontalPodAutoscaler(func(d *autoscaling.HorizontalPodAutoscaler) {
		d.Name = testName
		d.Namespace = testNamespace
	})).Build()

	err := DeleteHorizontalPodAutoscaler(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingHorizontalPodAutoscaler)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteHorizontalPodAutoscaler(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
