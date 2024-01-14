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
)

type configMapOpt func(*corev1.ConfigMap)

func getTestConfigMap(opts ...configMapOpt) *corev1.ConfigMap {
	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Data: make(map[string]string),
	}

	for _, opt := range opts {
		opt(desiredConfigMap)
	}
	return desiredConfigMap
}

func TestRequestConfigMap(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name             string
		deployReq        ConfigMapRequest
		desiredConfigMap *corev1.ConfigMap
		wantErr          bool
	}{
		{
			name: "request configMap, no mutation, custom name, labels, annotations",
			deployReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Data: testKVP,
			},
			desiredConfigMap: getTestConfigMap(func(cm *corev1.ConfigMap) {
				cm.Name = testName
				cm.Namespace = testNamespace
				cm.Labels = testKVP
				cm.Annotations = testKVP
				cm.Data = testKVP
			}),
			wantErr: false,
		},
		{
			name: "request configMap, successful mutation",
			deployReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Data: testKVP,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredConfigMap: getTestConfigMap(func(cm *corev1.ConfigMap) {
				cm.Name = testNameMutated
				cm.Namespace = testNamespace
				cm.Labels = testKVP
				cm.Annotations = testKVP
				cm.Data = testKVPMutated
			}),
			wantErr: false,
		},
		{
			name: "request configMap, failed mutation",
			deployReq: ConfigMapRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Data: testKVP,
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredConfigMap: getTestConfigMap(func(cm *corev1.ConfigMap) {
				cm.Name = testName
				cm.Namespace = testNamespace
				cm.Labels = testKVP
				cm.Annotations = testKVP
				cm.Data = testKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotConfigMap, err := RequestConfigMap(test.deployReq)

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

	desiredConfigMap := getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.TypeMeta = metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}
		cm.Name = testName
		cm.Namespace = testNamespace
		cm.Labels = testKVP
		cm.Annotations = testKVP
		cm.Data = testKVP
	})
	err := CreateConfigMap(desiredConfigMap, testClient)
	assert.NoError(t, err)

	createdConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdConfigMap)

	assert.NoError(t, err)
	assert.Equal(t, desiredConfigMap, createdConfigMap)
}

func TestGetConfigMap(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = testName
		cm.Namespace = testNamespace
		cm.Data = testKVP

	})).Build()

	_, err := GetConfigMap(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetConfigMap(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListConfigMaps(t *testing.T) {
	configMap1 := getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-1"
		cm.Namespace = testNamespace
		cm.Labels[common.AppK8sKeyComponent] = "new-component-1"
		cm.Data = testKVP

	})
	configMap2 := getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-2"
		cm.Namespace = testNamespace
	})
	configMap3 := getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = "configMap-3"
		cm.Labels[common.AppK8sKeyComponent] = "new-component-2"
		cm.Data = testKVP
		cm.Namespace = testNamespace
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

	existingConfigMapList, err := ListConfigMaps(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingConfigMaps := []string{}
	for _, configMap := range existingConfigMapList.Items {
		existingConfigMaps = append(existingConfigMaps, configMap.Name)
	}
	sort.Strings(existingConfigMaps)

	assert.Equal(t, desiredConfigMaps, existingConfigMaps)
}

func TestUpdateConfigMap(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = testName
		cm.Namespace = testNamespace
		cm.Data = testKVP
	})).Build()

	desiredConfigMap := getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = testName
		cm.Namespace = testNamespace
		cm.Data = map[string]string{
			"application.instanceLabelKey": "mycompany.com/appname",
			"admin.enabled":                "true",
		}
	})
	err := UpdateConfigMap(desiredConfigMap, testClient)
	assert.NoError(t, err)

	existingConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingConfigMap)

	assert.NoError(t, err)
	assert.Equal(t, desiredConfigMap.Data, existingConfigMap.Data)

	testClient = fake.NewClientBuilder().Build()
	existingConfigMap = getTestConfigMap(func(cm *corev1.ConfigMap) {
		cm.Name = testName
		cm.Namespace = testNamespace
		cm.Data = nil
	})
	err = UpdateConfigMap(existingConfigMap, testClient)
	assert.Error(t, err)
}

func TestDeleteConfigMap(t *testing.T) {
	testConfigMap := getTestConfigMap(func(configMap *corev1.ConfigMap) {
		configMap.Name = testName
		configMap.Namespace = testNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testConfigMap).Build()

	err := DeleteConfigMap(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingConfigMap := &corev1.ConfigMap{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingConfigMap)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
