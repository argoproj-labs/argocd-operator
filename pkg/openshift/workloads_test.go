package openshift

import (
	"context"
	"sort"
	"testing"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
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

type deploymentConfigOpt func(*oappsv1.DeploymentConfig)

func getTestDeploymentConfig(opts ...deploymentConfigOpt) *oappsv1.DeploymentConfig {
	desiredDeploymentConfig := &oappsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: oappsv1.DeploymentConfigSpec{},
	}

	for _, opt := range opts {
		opt(desiredDeploymentConfig)
	}
	return desiredDeploymentConfig
}

func TestRequestDeploymentConfig(t *testing.T) {

	s := scheme.Scheme
	assert.NoError(t, oappsv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	tests := []struct {
		name                    string
		deploymentConfigReq     DeploymentConfigRequest
		desiredDeploymentConfig *oappsv1.DeploymentConfig
		wantErr                 bool
	}{
		{
			name: "request deploymentConfig, no mutation, custom name, labels, annotations",
			deploymentConfigReq: DeploymentConfigRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: oappsv1.DeploymentConfigSpec{
					Selector: testKVP,
				},
			},
			desiredDeploymentConfig: getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
				dc.Name = testName
				dc.Namespace = testNamespace
				dc.Labels = testKVP
				dc.Annotations = testKVP
				dc.Spec.Selector = testKVP
			}),
			wantErr: false,
		},
		{
			name: "request deploymentConfig, successful mutation",
			deploymentConfigReq: DeploymentConfigRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: oappsv1.DeploymentConfigSpec{
					Selector: testKVP,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredDeploymentConfig: getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
				dc.Name = testNameMutated
				dc.Namespace = testNamespace
				dc.Labels = testKVP
				dc.Annotations = testKVP
				dc.Spec.Selector = testKVP
				dc.Spec.Replicas = testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request deploymentConfig, failed mutation",
			deploymentConfigReq: DeploymentConfigRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: oappsv1.DeploymentConfigSpec{
					Selector: testKVP,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredDeploymentConfig: getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {}),
			wantErr:                 true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotDeploymentConfig, err := RequestDeploymentConfig(test.deploymentConfigReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredDeploymentConfig, gotDeploymentConfig)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateDeploymentConfig(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, oappsv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	desiredDeploymentConfig := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.TypeMeta = metav1.TypeMeta{
			Kind:       "DeploymentConfig",
			APIVersion: "apps.openshift.io/v1",
		}
		dc.Name = testName
		dc.Namespace = testNamespace
		dc.Labels = testKVP
		dc.Annotations = testKVP
	})
	err := CreateDeploymentConfig(desiredDeploymentConfig, testClient)
	assert.NoError(t, err)

	createdDeploymentConfig := &oappsv1.DeploymentConfig{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdDeploymentConfig)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeploymentConfig, createdDeploymentConfig)
}

func TestGetDeploymentConfig(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, oappsv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = testName
		dc.Namespace = testNamespace
	})).Build()

	_, err := GetDeploymentConfig(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetDeploymentConfig(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListDeploymentConfigs(t *testing.T) {
	deploymentConfig1 := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = "deploymentConfig-1"
		dc.Namespace = testNamespace
		dc.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	deploymentConfig2 := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) { dc.Name = "deploymentConfig-2" })
	deploymentConfig3 := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = "deploymentConfig-3"
		dc.Namespace = testNamespace
		dc.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, oappsv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		deploymentConfig1, deploymentConfig2, deploymentConfig3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredDeploymentConfigs := []string{"deploymentConfig-1", "deploymentConfig-3"}

	existingDeploymentConfigList, err := ListDeploymentConfigs(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingDeploymentConfigs := []string{}
	for _, deploymentConfig := range existingDeploymentConfigList.Items {
		existingDeploymentConfigs = append(existingDeploymentConfigs, deploymentConfig.Name)
	}
	sort.Strings(existingDeploymentConfigs)

	assert.Equal(t, desiredDeploymentConfigs, existingDeploymentConfigs)
}

func TestUpdateDeploymentConfig(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, oappsv1.AddToScheme(s))

	// Create the initial DeploymentConfig
	initialDeploymentConfig := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = testName
		dc.Namespace = testNamespace
	})

	// Create the client with the initial DeploymentConfig
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialDeploymentConfig).Build()

	// Fetch the DeploymentConfig from the client
	desiredDeploymentConfig := &oappsv1.DeploymentConfig{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: testName, Namespace: testNamespace}, desiredDeploymentConfig)
	assert.NoError(t, err)
	desiredDeploymentConfig.Labels = map[string]string{
		"control-plane": "argocd-operator",
	}

	err = UpdateDeploymentConfig(desiredDeploymentConfig, testClient)
	assert.NoError(t, err)

	existingDeploymentConfig := &oappsv1.DeploymentConfig{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingDeploymentConfig)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeploymentConfig.Labels, existingDeploymentConfig.Labels)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingDeploymentConfig = getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = testName
		dc.Labels = nil
	})
	err = UpdateDeploymentConfig(existingDeploymentConfig, testClient)
	assert.Error(t, err)
}

func TestDeleteDeploymentConfig(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, routev1.AddToScheme(s))

	testDeploymentConfig := getTestDeploymentConfig(func(dc *oappsv1.DeploymentConfig) {
		dc.Name = testName
		dc.Namespace = testNamespace
	})

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(testDeploymentConfig).Build()

	err := DeleteDeploymentConfig(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingDeploymentConfig := &oappsv1.DeploymentConfig{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingDeploymentConfig)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
