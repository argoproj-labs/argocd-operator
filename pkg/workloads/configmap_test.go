package workloads

import (
	"context"
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

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestConfigMap(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name             string
		cmReq            ConfigMapRequest
		desiredConfigMap *corev1.ConfigMap
		wantErr          bool
	}{
		{
			name: "request configMap",
			cmReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Data: test.TestKVP,
			},
			desiredConfigMap: test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
				cm.Name = test.TestName
				cm.Namespace = test.TestNamespace
				cm.Labels = test.TestKVP
				cm.Annotations = test.TestKVP
				cm.Data = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request configMap, successful mutation",
			cmReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Data: test.TestKVP,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredConfigMap: test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
				cm.Name = test.TestNameMutated
				cm.Namespace = test.TestNamespace
				cm.Labels = test.TestKVP
				cm.Annotations = test.TestKVP
				cm.Data = test.TestKVPMutated
			}),
			wantErr: false,
		},
		{
			name: "request configMap, failed mutation",
			cmReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Data: test.TestKVP,
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredConfigMap: test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
				cm.Name = test.TestName
				cm.Namespace = test.TestNamespace
				cm.Labels = test.TestKVP
				cm.Annotations = test.TestKVP
				cm.Data = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotConfigMap, err := RequestConfigMap(test.cmReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredConfigMap, gotConfigMap)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateConfigMap(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredConfigMap := test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.TypeMeta = metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}
		cm.Name = test.TestName
		cm.Namespace = test.TestNamespace
		cm.Labels = test.TestKVP
		cm.Annotations = test.TestKVP
		cm.Data = test.TestKVP
	})
	err := CreateConfigMap(desiredConfigMap, testClient)
	assert.NoError(t, err)

	createdConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdConfigMap)

	assert.NoError(t, err)
	assert.Equal(t, desiredConfigMap, createdConfigMap)
}

func TestGetConfigMap(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = test.TestName
		cm.Namespace = test.TestNamespace
		cm.Data = test.TestKVP

	})).Build()

	_, err := GetConfigMap(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetConfigMap(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListConfigMaps(t *testing.T) {
	configMap1 := test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-1"
		cm.Namespace = test.TestNamespace
		cm.Labels[common.AppK8sKeyComponent] = "new-component-1"
		cm.Data = test.TestKVP

	})
	configMap2 := test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-2"
		cm.Namespace = test.TestNamespace
	})
	configMap3 := test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-3"
		cm.Labels[common.AppK8sKeyComponent] = "new-component-2"
		cm.Data = test.TestKVP
		cm.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		configMap1, configMap2, configMap3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredConfigMaps := []string{"configMap-1", "configMap-3"}

	existingConfigMapList, err := ListConfigMaps(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingConfigMaps := []string{}
	for _, configMap := range existingConfigMapList.Items {
		existingConfigMaps = append(existingConfigMaps, configMap.Name)
	}
	sort.Strings(existingConfigMaps)

	assert.Equal(t, desiredConfigMaps, existingConfigMaps)
}

func TestUpdateConfigMap(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = test.TestName
		cm.Namespace = test.TestNamespace
		cm.Data = test.TestKVP
	})).Build()

	desiredConfigMap := test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = test.TestName
		cm.Namespace = test.TestNamespace
		cm.Data = map[string]string{
			"application.instanceLabelKey": "mycompany.com/appname",
			"admin.enabled":                "true",
		}
	})
	err := UpdateConfigMap(desiredConfigMap, testClient)
	assert.NoError(t, err)

	existingConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingConfigMap)

	assert.NoError(t, err)
	assert.Equal(t, desiredConfigMap.Data, existingConfigMap.Data)

	testClient = fake.NewClientBuilder().Build()
	existingConfigMap = test.MakeTestConfigMap(nil, func(cm *corev1.ConfigMap) {
		cm.Name = test.TestName
		cm.Namespace = test.TestNamespace
		cm.Data = nil
	})
	err = UpdateConfigMap(existingConfigMap, testClient)
	assert.Error(t, err)
}

func TestDeleteConfigMap(t *testing.T) {
	testConfigMap := test.MakeTestConfigMap(nil, func(configMap *corev1.ConfigMap) {
		configMap.Name = test.TestName
		configMap.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testConfigMap).Build()

	err := DeleteConfigMap(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingConfigMap)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
