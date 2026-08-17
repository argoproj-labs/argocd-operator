// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gitopspromoter handles reconcilation related to the GitOps Promoter
package gitopspromoter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func makeExistingDeployment(sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) *appsv1.Deployment {
	env := []corev1.EnvVar{}
	if cr.Spec.Promoter != nil {
		env = cr.Spec.Promoter.Env
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(testCompName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(testCompName, cr),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: buildLabelsForPromoterResources(testCompName, cr),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: buildLabelsForPromoterResources(testCompName, cr),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command:         buildContainerCommand("test"),
							Image:           selectImage(cr),
							ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
							Name:            generatePromoterResourceName(testCompName, cr),
							Env:             env,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: 6443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: getResources(cr),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(9081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(9081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume",
									MountPath: "/home/test",
								},
							},
						},
					},
					ServiceAccountName:            sa.Name,
					TerminationGracePeriodSeconds: ptr.To(int64(10)),
					Volumes: []corev1.Volume{
						{
							Name: "test-vol",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "test-secret",
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestReconcilePromoterDeployment_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and Deployment does not exist
	// Expected: Deployment should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterDeployment_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and Deployment does not exist
	// Expected: Deployment should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.NoError(t, err)

	assert.Equal(t, referenceDeployment.Name, retrievedDeployment.Name)
	assert.Equal(t, referenceDeployment.Namespace, retrievedDeployment.Namespace)
	assert.Equal(t, referenceDeployment.Labels, retrievedDeployment.Labels)

	assert.Equal(t, referenceDeployment.Spec.Selector, retrievedDeployment.Spec.Selector)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Command, retrievedDeployment.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Image, retrievedDeployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy, retrievedDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Name, retrievedDeployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Env, retrievedDeployment.Spec.Template.Spec.Containers[0].Env)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext, retrievedDeployment.Spec.Template.Spec.Containers[0].SecurityContext)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Resources, retrievedDeployment.Spec.Template.Spec.Containers[0].Resources)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Ports, retrievedDeployment.Spec.Template.Spec.Containers[0].Ports)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].LivenessProbe)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, retrievedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.ServiceAccountName, retrievedDeployment.Spec.Template.Spec.ServiceAccountName)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds, retrievedDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Volumes, retrievedDeployment.Spec.Template.Spec.Volumes)
}

func TestReconcilePromoterDeployment_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and Deployment exists
	// Expected: Deployment should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	resObjs := []client.Object{cr, referenceDeployment}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterDeployment_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and Deployment exists
	// Expected: Deployment should be deleted

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	resObjs := []client.Object{cr, referenceDeployment}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterDeployment_DoesNotExists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and Deployment does not exists
	// Expected: Deployment should not be created

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterDeployment_Exists_PromoterEnabled_Update(t *testing.T) {
	// Test Case: Promoter is enabled and Deployment exists but has been updated
	// Expected: Deployment should be updated to match the desired state

	cr := makeTestArgoCD(withPromoterEnabled(true))
	cr.Spec.Promoter.Env = []corev1.EnvVar{
		{
			Name:  "ENV_VARIABLE_THAT_SHOULD_BE_SET",
			Value: "true",
		},
	}
	cr.Spec.Promoter.Resources = &corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	sa := makeExistingServiceAccount(cr)
	referenceDeployment := makeExistingDeployment(sa, cr)
	cfg := deploymentConfig{
		command:         referenceDeployment.Spec.Template.Spec.Containers[0].Command,
		securityContext: referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext,
		ports:           referenceDeployment.Spec.Template.Spec.Containers[0].Ports,
		livenessProbe:   referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe,
		readinessProbe:  referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe,
		volumes:         referenceDeployment.Spec.Template.Spec.Volumes,
		volumeMounts:    referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
	}

	updatedDeployment := referenceDeployment.DeepCopy()
	updatedDeployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"key": "value",
		},
	}
	updatedDeployment.Spec.Template.Spec.Containers[0].Command = []string{"cowsay"}
	updatedDeployment.Spec.Template.Spec.Containers[0].Image = "quay.io/someuser-that-probably-does-not-exist/gitops-promoter"
	updatedDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullNever
	updatedDeployment.Spec.Template.Spec.Containers[0].Name = "random-name-that-is-not-correct"
	updatedDeployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "RANDOM_ENV_VARIABLE",
			Value: "should-not-be-set",
		},
	}
	updatedDeployment.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(true),
	}
	updatedDeployment.Spec.Template.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("99"),
			corev1.ResourceMemory: resource.MustParse("1024Mi"),
		},
	}
	updatedDeployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			Name:          "not the right port",
			Protocol:      corev1.ProtocolUDP,
			ContainerPort: 555,
		},
	}
	updatedDeployment.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{}
	updatedDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{}
	updatedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{}
	updatedDeployment.Spec.Template.Spec.ServiceAccountName = "not a real service account"
	updatedDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds = ptr.To(int64(999))
	updatedDeployment.Spec.Template.Spec.Volumes = []corev1.Volume{}

	resObjs := []client.Object{cr, updatedDeployment}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterDeployment(client, testCompName, sa, cr, sch, cfg, true)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.NoError(t, err)

	assert.Equal(t, referenceDeployment.Name, retrievedDeployment.Name)
	assert.Equal(t, referenceDeployment.Namespace, retrievedDeployment.Namespace)
	assert.Equal(t, referenceDeployment.Labels, retrievedDeployment.Labels)

	assert.Equal(t, referenceDeployment.Spec.Selector, retrievedDeployment.Spec.Selector)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Command, retrievedDeployment.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Image, retrievedDeployment.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy, retrievedDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Name, retrievedDeployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Env, retrievedDeployment.Spec.Template.Spec.Containers[0].Env)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].SecurityContext, retrievedDeployment.Spec.Template.Spec.Containers[0].SecurityContext)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Resources, retrievedDeployment.Spec.Template.Spec.Containers[0].Resources)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].Ports, retrievedDeployment.Spec.Template.Spec.Containers[0].Ports)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].LivenessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].LivenessProbe)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, retrievedDeployment.Spec.Template.Spec.Containers[0].VolumeMounts)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.ServiceAccountName, retrievedDeployment.Spec.Template.Spec.ServiceAccountName)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds, retrievedDeployment.Spec.Template.Spec.TerminationGracePeriodSeconds)
	assert.Equal(t, referenceDeployment.Spec.Template.Spec.Volumes, retrievedDeployment.Spec.Template.Spec.Volumes)
}

func TestReconcilePromoterControllerDeployment_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and Controller Deployment does not exist
	// Expected: Controller Deployment should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterControllerDeployment(client, testCompName, sa, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      deployment.Name,
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerDeployment_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and Controller Deployment does not exist
	// Expected: Controller Deployment should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterControllerDeployment(client, testCompName, sa, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      deployment.Name,
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.NoError(t, err)

	assert.Equal(t, deployment.Name, retrievedDeployment.Name)
	assert.Equal(t, cr.Namespace, retrievedDeployment.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedDeployment.Labels)

	cfg := createControllerConfig()
	assert.Equal(t, cfg.command, retrievedDeployment.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, cfg.securityContext, retrievedDeployment.Spec.Template.Spec.Containers[0].SecurityContext)
	assert.Equal(t, cfg.livenessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].LivenessProbe)
	assert.Equal(t, cfg.readinessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
}

func TestReconcilePromoterAPIServerDeployment_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and API Server Deployment does not exist
	// Expected: API Server Deployment should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterAPIServerDeployment(client, testCompName, sa, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      deployment.Name,
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerDeployment_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and API Server Deployment does not exist
	// Expected: API Server Deployment should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterAPIServerDeployment(client, testCompName, sa, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      deployment.Name,
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.NoError(t, err)

	assert.Equal(t, deployment.Name, retrievedDeployment.Name)
	assert.Equal(t, cr.Namespace, retrievedDeployment.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedDeployment.Labels)

	cfg := createAPIServerConfig()
	assert.Equal(t, cfg.command, retrievedDeployment.Spec.Template.Spec.Containers[0].Command)
	assert.Equal(t, cfg.args, retrievedDeployment.Spec.Template.Spec.Containers[0].Args)
	assert.Equal(t, cfg.securityContext, retrievedDeployment.Spec.Template.Spec.Containers[0].SecurityContext)
	assert.Equal(t, cfg.ports, retrievedDeployment.Spec.Template.Spec.Containers[0].Ports)
	assert.Equal(t, cfg.livenessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].LivenessProbe)
	assert.Equal(t, cfg.readinessProbe, retrievedDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe)
}

func TestReconcilePromoterAPIServerDeployment_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test Case: Promoter is enabled and API Server is disabled
	// Expected: API Server Deployment should not be created

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	deployment, err := ReconcilePromoterAPIServerDeployment(client, testCompName, sa, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, deployment)

	retrievedDeployment := &appsv1.Deployment{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      deployment.Name,
		Namespace: cr.Namespace,
	}, retrievedDeployment)
	assert.True(t, errors.IsNotFound(err))
}
