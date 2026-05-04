package argocd

import (
	"context"
	"strings"
	"testing"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func TestReconcileArgoCD_reconcileDexDeployment_with_dex_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name       string
		setEnvFunc func(*testing.T, string)
		argoCD     *argoproj.ArgoCD
	}{
		{
			name:       "dex disabled by not specifying .spec.sso.provider=dex",
			setEnvFunc: nil,
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "true")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			err := r.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)
			assert.True(t, apierrors.IsNotFound(err))
		})
	}
}

// When Dex is enabled dex deployment should be created, when disabled the Dex deployment should be removed
func TestReconcileArgoCD_reconcileDexDeployment_removes_dex_when_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                  string
		setEnvFunc            func(*testing.T, string)
		updateCrFunc          func(cr *argoproj.ArgoCD)
		updateEnvFunc         func(*testing.T, string)
		argoCD                *argoproj.ArgoCD
		wantDeploymentDeleted bool
	}{
		{
			name:       "dex disabled by removing .spec.sso",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantDeploymentDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			// ensure deployment was created correctly
			deployment := &appsv1.Deployment{}
			err := r.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))
			deployment = &appsv1.Deployment{}
			err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)

			if test.wantDeploymentDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReconcileArgoCD_reconcileDeployments_Dex_with_resources(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name       string
		setEnvFunc func(*testing.T, string)
		argoCD     *argoproj.ArgoCD
	}{
		{
			name:       "dex with resources - .spec.sso.provider=dex",
			setEnvFunc: nil,
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
								corev1.ResourceCPU:    resourcev1.MustParse("250m"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
								corev1.ResourceCPU:    resourcev1.MustParse("500m"),
							},
						},
					},
				}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      test.argoCD.Name + "-dex-server",
					Namespace: test.argoCD.Namespace,
				},
				deployment))

			testResources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
					corev1.ResourceCPU:    resourcev1.MustParse("250m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
					corev1.ResourceCPU:    resourcev1.MustParse("500m"),
				},
			}
			assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Resources, testResources)
			assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].Resources, testResources)
		})
	}
}

func TestReconcileArgoCD_reconcileDeployments_Dex_with_volumes(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name       string
		setEnvFunc func(*testing.T, string)
		argoCD     *argoproj.ArgoCD
	}{
		{
			name:       "dex with volumes - .spec.sso.provider=dex",
			setEnvFunc: nil,
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Volumes: []corev1.Volume{
							{Name: "custom-config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "custom-config", MountPath: "/etc/custom-config"},
						},
					},
				}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      test.argoCD.Name + "-dex-server",
					Namespace: test.argoCD.Namespace,
				},
				deployment))

			testVolumes := []corev1.Volume{
				{
					Name: "static-files",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "dexconfig",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "custom-config",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			}

			testVolumeMounts := []corev1.VolumeMount{
				{Name: "static-files", MountPath: "/shared"},
				{Name: "dexconfig", MountPath: "/tmp"},
				{Name: "custom-config", MountPath: "/etc/custom-config"},
			}

			assert.Equal(t, deployment.Spec.Template.Spec.Volumes, testVolumes)

			assert.Equal(t, deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts, testVolumeMounts)
			assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts, testVolumeMounts)
		})
	}
}

func TestReconcileArgoCD_reconcileDexDeployment(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-dex-server",
			Namespace: a.Namespace,
		},
		deployment))
	want := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "static-files",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "dexconfig",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
		InitContainers: []corev1.Container{
			{
				Name:  "copyutil",
				Image: getArgoContainerImage(a),
				Command: []string{
					"cp",
					"-n",
					"/usr/local/bin/argocd",
					"/shared/argocd-dex",
				},
				SecurityContext: argoutil.DefaultSecurityContext(),
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "static-files",
						MountPath: "/shared",
					},
					{
						Name:      "dexconfig",
						MountPath: "/tmp",
					},
				},
				ImagePullPolicy: corev1.PullIfNotPresent,
			},
		},
		Containers: []corev1.Container{
			{
				Name:  "dex",
				Image: getDexContainerImage(a),
				Command: []string{
					"/shared/argocd-dex",
					"rundex",
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz/live",
							Port: intstr.FromInt(5558),
						},
					},
					InitialDelaySeconds: 60,
					PeriodSeconds:       30,
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: 5556,
					},
					{
						Name:          "grpc",
						ContainerPort: 5557,
					},
					{
						Name:          "metrics",
						ContainerPort: 5558,
					},
				},
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: argoutil.DefaultSecurityContext(),
				VolumeMounts: []corev1.VolumeMount{
					{Name: "static-files", MountPath: "/shared"},
					{Name: "dexconfig", MountPath: "/tmp"},
				},
			},
		},
		ServiceAccountName: "argocd-argocd-dex-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}
	assert.Equal(t, want, deployment.Spec.Template.Spec)
}

func TestReconcileArgoCD_reconcileDexDeployment_withUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name         string
		setEnvFunc   func(*testing.T, string)
		updateCrFunc func(cr *argoproj.ArgoCD)
		argoCD       *argoproj.ArgoCD
		wantPodSpec  corev1.PodSpec
	}{
		{
			name:       "update dex deployment - .spec.sso.provider=dex + .spec.sso.dex",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.Image = "justatest"
				cr.Spec.Version = "latest"
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Image:   "testdex",
						Version: "v0.0.1",
					},
				}
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantPodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "static-files",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "dexconfig",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:  "copyutil",
						Image: "justatest:latest",
						Command: []string{
							"cp",
							"-n",
							"/usr/local/bin/argocd",
							"/shared/argocd-dex",
						},
						SecurityContext: argoutil.DefaultSecurityContext(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "static-files",
								MountPath: "/shared",
							},
							{
								Name:      "dexconfig",
								MountPath: "/tmp",
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "dex",
						Image: "testdex:v0.0.1",
						Command: []string{
							"/shared/argocd-dex",
							"rundex",
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz/live",
									Port: intstr.FromInt(5558),
								},
							},
							InitialDelaySeconds: 60,
							PeriodSeconds:       30,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 5556,
							},
							{
								Name:          "grpc",
								ContainerPort: 5557,
							},
							{
								Name:          "metrics",
								ContainerPort: 5558,
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: argoutil.DefaultSecurityContext(),
						VolumeMounts: []corev1.VolumeMount{
							{Name: "static-files", MountPath: "/shared"},
							{Name: "dexconfig", MountPath: "/tmp"},
						},
					},
				},
				ServiceAccountName: "argocd-argocd-dex-server",
				NodeSelector:       common.DefaultNodeSelector(),
			},
		},
		{
			name:       "update dex deployment - .spec.sso.dex.env",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO.Dex.Env = []corev1.EnvVar{
					{
						Name: "ARGO_WORKFLOWS_SSO_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "argo-workflows-sso",
								},
								Key: "client-secret",
							},
						},
					},
				}
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantPodSpec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "static-files",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "dexconfig",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:  "copyutil",
						Image: "quay.io/argoproj/argocd@" + common.ArgoCDDefaultArgoVersion,
						Command: []string{
							"cp",
							"-n",
							"/usr/local/bin/argocd",
							"/shared/argocd-dex",
						},
						SecurityContext: argoutil.DefaultSecurityContext(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "static-files",
								MountPath: "/shared",
							},
							{
								Name:      "dexconfig",
								MountPath: "/tmp",
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
				Containers: []corev1.Container{
					{
						Name:  "dex",
						Image: "ghcr.io/dexidp/dex@sha256:b08a58c9731c693b8db02154d7afda798e1888dc76db30d34c4a0d0b8a26d913", // (v2.43.0) NOTE: this value is modified by dependency update script
						Command: []string{
							"/shared/argocd-dex",
							"rundex",
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz/live",
									Port: intstr.FromInt(5558),
								},
							},
							InitialDelaySeconds: 60,
							PeriodSeconds:       30,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								ContainerPort: 5556,
							},
							{
								Name:          "grpc",
								ContainerPort: 5557,
							},
							{
								Name:          "metrics",
								ContainerPort: 5558,
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "ARGO_WORKFLOWS_SSO_CLIENT_SECRET",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "argo-workflows-sso",
										},
										Key: "client-secret",
									},
								},
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: argoutil.DefaultSecurityContext(),
						VolumeMounts: []corev1.VolumeMount{
							{Name: "static-files", MountPath: "/shared"},
							{Name: "dexconfig", MountPath: "/tmp"},
						},
					},
				},
				ServiceAccountName: "argocd-argocd-dex-server",
				NodeSelector:       common.DefaultNodeSelector(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			// ensure deployment was created correctly
			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-dex-server",
					Namespace: test.argoCD.Namespace,
				},
				deployment))

			assert.Equal(t, test.wantPodSpec, deployment.Spec.Template.Spec)
		})
	}
}

func TestReconcileArgoCD_reconcileDexDeployment_updatesInitContainerFields(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// staleDeployment simulates a Dex Deployment created by an old operator version (e.g., v1.13)
	// that only has the "static-files" volumeMount in the copyutil initContainer and no dexconfig volume.
	staleDeployment := func(namespace string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-dex-server",
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app.kubernetes.io/name": "argocd-dex-server",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app.kubernetes.io/name": "argocd-dex-server",
						},
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name: "copyutil",
								// Old state: only static-files, dexconfig mount is absent
								VolumeMounts: []corev1.VolumeMount{
									{Name: "static-files", MountPath: "/shared"},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resourcev1.MustParse("15m"),
										corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
									},
								},
							},
						},
						Containers: []corev1.Container{
							{Name: "dex"},
						},
						Volumes: []corev1.Volume{
							{
								Name:         "static-files",
								VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name                 string
		argoCD               *argoproj.ArgoCD
		wantInitVolumeMounts []corev1.VolumeMount
		wantInitResources    corev1.ResourceRequirements
	}{
		{
			name: "reconciler adds missing dexconfig volumeMount to copyutil initContainer on upgrade",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
				}
			}),
			wantInitVolumeMounts: []corev1.VolumeMount{
				{Name: "static-files", MountPath: "/shared"},
				{Name: "dexconfig", MountPath: "/tmp"},
			},
			wantInitResources: corev1.ResourceRequirements{},
		},
		{
			name: "reconciler updates stale resources in copyutil initContainer on upgrade",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
								corev1.ResourceCPU:    resourcev1.MustParse("250m"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
								corev1.ResourceCPU:    resourcev1.MustParse("500m"),
							},
						},
					},
				}
			}),
			wantInitVolumeMounts: []corev1.VolumeMount{
				{Name: "static-files", MountPath: "/shared"},
				{Name: "dexconfig", MountPath: "/tmp"},
			},
			wantInitResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
					corev1.ResourceCPU:    resourcev1.MustParse("250m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
					corev1.ResourceCPU:    resourcev1.MustParse("500m"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			existingDeploy := staleDeployment(test.argoCD.Namespace)
			resObjs := []client.Object{test.argoCD, existingDeploy}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-dex-server",
					Namespace: test.argoCD.Namespace,
				},
				deployment))

			assert.Equal(t, test.wantInitVolumeMounts,
				deployment.Spec.Template.Spec.InitContainers[0].VolumeMounts,
				"copyutil initContainer VolumeMounts should be updated by reconciler")
			assert.Equal(t, test.wantInitResources,
				deployment.Spec.Template.Spec.InitContainers[0].Resources,
				"copyutil initContainer Resources should be updated by reconciler")
		})
	}
}

// When Dex is enabled dex service should be created, when disabled the Dex service should be removed
func TestReconcileArgoCD_reconcileDexService_removes_dex_when_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name               string
		setEnvFunc         func(*testing.T, string)
		updateCrFunc       func(cr *argoproj.ArgoCD)
		updateEnvFunc      func(*testing.T, string)
		argoCD             *argoproj.ArgoCD
		wantServiceDeleted bool
	}{
		{
			name:       "dex disabled by removing .spec.sso",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantServiceDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexService(test.argoCD))

			// ensure service was created correctly
			service := &corev1.Service{}
			err := r.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, service)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			assert.NoError(t, r.reconcileDexService(test.argoCD))
			service = &corev1.Service{}
			err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, service)

			if test.wantServiceDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// When Dex is enabled dex serviceaccount should be created, when disabled the Dex serviceaccount should be removed
func TestReconcileArgoCD_reconcileDexServiceAccount_removes_dex_when_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                      string
		setEnvFunc                func(*testing.T, string)
		updateCrFunc              func(cr *argoproj.ArgoCD)
		updateEnvFunc             func(*testing.T, string)
		argoCD                    *argoproj.ArgoCD
		wantServiceAccountDeleted bool
	}{
		{
			name:       "dex disabled by removing .spec.sso",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantServiceAccountDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			sa, err := r.reconcileServiceAccount(common.ArgoCDDexServerComponent, test.argoCD)
			assert.NoError(t, err)

			// ensure serviceaccount was created correctly
			err = r.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: test.argoCD.Namespace}, sa)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			_, err = r.reconcileServiceAccount(common.ArgoCDDexServerComponent, test.argoCD)
			assert.NoError(t, err)

			err = r.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: test.argoCD.Namespace}, sa)

			if test.wantServiceAccountDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// When Dex is enabled dex role should be created, when disabled the Dex role should be removed
func TestReconcileArgoCD_reconcileRole_dex_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name            string
		setEnvFunc      func(*testing.T, string)
		updateCrFunc    func(cr *argoproj.ArgoCD)
		updateEnvFunc   func(*testing.T, string)
		argoCD          *argoproj.ArgoCD
		wantRoleDeleted bool
	}{
		{
			name:       "dex disabled by removing .spec.sso",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantRoleDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			rules := policyRuleForDexServer()
			role := newRole(common.ArgoCDDexServerComponent, rules, test.argoCD)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			_, err := r.reconcileRole(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			// ensure role was created correctly
			err = r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: test.argoCD.Namespace}, role)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			_, err = r.reconcileRole(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			err = r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: test.argoCD.Namespace}, role)

			if test.wantRoleDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// When Dex is enabled dex roleBinding should be created, when disabled the Dex roleBinding should be removed
func TestReconcileArgoCD_reconcileRoleBinding_dex_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                   string
		setEnvFunc             func(*testing.T, string)
		updateCrFunc           func(cr *argoproj.ArgoCD)
		updateEnvFunc          func(*testing.T, string)
		argoCD                 *argoproj.ArgoCD
		wantRoleBindingDeleted bool
	}{
		{
			name:       "dex disabled by removing .spec.sso",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantRoleBindingDeleted: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			rules := policyRuleForDexServer()
			roleBinding := newRoleBindingWithname(common.ArgoCDDexServerComponent, test.argoCD)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileRoleBinding(common.ArgoCDDexServerComponent, rules, test.argoCD))
			assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding))

			// ensure roleBinding was created correctly
			err := r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			err = r.reconcileRoleBinding(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			err = r.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding)

			if test.wantRoleBindingDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsExternalAuthenticationEnabledOnCluster(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		expected bool
	}{
		{
			name:     "OIDC enabled",
			authType: "OIDC",
			expected: true,
		},
		{
			name:     "Non OIDC type",
			authType: "SSO",
			expected: false,
		},
		{
			name:     "Empty type",
			authType: "",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster",
				},
				Spec: configv1.AuthenticationSpec{
					Type: configv1.AuthenticationType(tt.authType),
				},
			}
			sch := makeTestReconcilerScheme(configv1.AddToScheme)
			cl := makeTestReconcilerClient(sch, []client.Object{auth}, []client.Object{auth}, nil)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			result := IsExternalAuthenticationEnabledOnCluster(context.TODO(), r.Client)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	require.NoError(t, configv1.AddToScheme(scheme))
	require.NoError(t, argoproj.AddToScheme(scheme))
	return scheme
}

func newTestClient(scheme *runtime.Scheme, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&argoproj.ArgoCD{}).WithObjects(objs...).Build()
}

// OIDC enabled on cluster. openshift cluster, status condition gets populated on argocd status
func TestGetOpenShiftDexConfig_StatusUpdateSuccess(t *testing.T) {
	original := versionAPIFound
	versionAPIFound = true
	defer func() { versionAPIFound = original }()
	scheme := newTestScheme(t)
	auth := &configv1.Authentication{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.AuthenticationSpec{
			Type: configv1.AuthenticationType("OIDC"),
		},
	}
	cr := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
	}
	cl := newTestClient(scheme, auth, cr)
	r := &ReconcileArgoCD{Client: cl}
	result, err := r.getOpenShiftDexConfig(cr)
	assert.NoError(t, err)
	assert.Empty(t, result)
	// Fetch updated CR
	updated := &argoproj.ArgoCD{}
	require.NoError(t, cl.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: "default"}, updated))
	require.NotEmpty(t, updated.Status.Conditions)
	found := false
	for _, cond := range updated.Status.Conditions {
		if cond.Type == argoproj.ArgoCDConditionConfigurationError {
			found = true
			assert.Equal(t, metav1.ConditionTrue, cond.Status)
			assert.Equal(t, argoproj.ArgoCDConditionReasonSSOError, cond.Reason)
			assert.Equal(t, argoproj.OpenShiftOAuthErrorMessage, cond.Message)
		}
	}
	assert.True(t, found, "expected configuration error condition")
}

// OIDC Disabled testcase
func TestGetOpenShiftDexConfig_OIDCDisabled(t *testing.T) {
	original := versionAPIFound
	versionAPIFound = true
	defer func() { versionAPIFound = original }()
	scheme := newTestScheme(t)
	auth := &configv1.Authentication{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: configv1.AuthenticationSpec{
			Type: configv1.AuthenticationType("SSO"),
		},
	}
	cr := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example",
			Namespace: "default",
		},
	}
	cl := newTestClient(scheme, auth, cr)
	r := &ReconcileArgoCD{Client: cl}
	result, err := r.getOpenShiftDexConfig(cr)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	updated := &argoproj.ArgoCD{}
	require.NoError(t, cl.Get(context.TODO(), types.NamespacedName{Name: "example", Namespace: "default"}, updated))
	assert.Empty(t, updated.Status.Conditions)
}

func TestNeedsDexTokenRenewal(t *testing.T) {
	renewThreshold := dexServerTokenRenewalThreshold()

	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{
			name:   "no expiry key - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{"token": []byte("t")}},
			want:   true,
		},
		{
			name:   "unparseable expiry - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{"expiry": []byte("not-a-time")}},
			want:   true,
		},
		{
			name: "expired token - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{
				"expiry": []byte(time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)),
			}},
			want: true,
		},
		{
			name: "within renewal window - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{
				// just inside the threshold (renewThreshold - 1s remaining)
				"expiry": []byte(time.Now().Add(renewThreshold - time.Second).UTC().Format(time.RFC3339)),
			}},
			want: true,
		},
		{
			name: "outside renewal window - no renewal needed",
			secret: &corev1.Secret{Data: map[string][]byte{
				"expiry": []byte(time.Now().Add(renewThreshold + time.Hour).UTC().Format(time.RFC3339)),
				"token":  []byte("present"),
			}},
			want: false,
		},
		{
			name:   "nil secret - needs renewal",
			secret: nil,
			want:   true,
		},
		{
			name: "valid expiry but missing token key - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{
				"expiry": []byte(time.Now().Add(renewThreshold + time.Hour).UTC().Format(time.RFC3339)),
			}},
			want: true,
		},
		{
			name: "valid expiry but empty token - needs renewal",
			secret: &corev1.Secret{Data: map[string][]byte{
				"expiry": []byte(time.Now().Add(renewThreshold + time.Hour).UTC().Format(time.RFC3339)),
				"token":  []byte{},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needsDexTokenRenewal(tt.secret))
		})
	}
}

func TestReconcileArgoCD_getDexOAuthClientSecret_ReturnsCachedToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	const firstToken = "first-token"
	const secondToken = "second-token"

	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex:      &argoproj.ArgoCDDexSpec{OpenShiftOAuth: true},
		}
	})

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, []client.Object{a}, []client.Object{a}, nil)

	// First call uses firstToken reactor.
	r := makeTestReconciler(cl, sch, makeTestK8sClientWithTokenReactor(firstToken))
	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	_, err := r.reconcileServiceAccount(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, err)

	token1, err := r.getDexOAuthClientSecret(a)
	assert.NoError(t, err)
	require.NotNil(t, token1)
	assert.Equal(t, firstToken, *token1)

	// Swap the K8sClient reactor to return a different token.
	// The cached Secret is still valid, so the same firstToken must be returned.
	r.K8sClient = makeTestK8sClientWithTokenReactor(secondToken)

	token2, err := r.getDexOAuthClientSecret(a)
	assert.NoError(t, err)
	require.NotNil(t, token2)
	assert.Equal(t, firstToken, *token2, "cached token should be returned while Secret is still valid")
}

func TestReconcileArgoCD_getDexOAuthClientSecret_RenewsExpiredToken(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	const expiredToken = "expired-token"
	const renewedToken = "renewed-token"

	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex:      &argoproj.ArgoCDDexSpec{OpenShiftOAuth: true},
		}
	})

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, []client.Object{a}, []client.Object{a}, nil)
	r := makeTestReconciler(cl, sch, makeTestK8sClientWithTokenReactor(renewedToken))
	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	_, err := r.reconcileServiceAccount(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, err)

	// Create an expired token Secret.
	expiredSecret := argoutil.NewSecretWithSuffix(a, common.ArgoCDDefaultDexServiceAccountName+"-token")
	expiredSecret.Type = corev1.SecretTypeOpaque
	expiredSecret.Data = map[string][]byte{
		"token":  []byte(expiredToken),
		"expiry": []byte(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)), // expired 1h ago
	}
	argoutil.AddTrackedByOperatorLabel(&expiredSecret.ObjectMeta)
	assert.NoError(t, r.Create(context.TODO(), expiredSecret))

	token, err := r.getDexOAuthClientSecret(a)
	assert.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, renewedToken, *token, "expired token must be replaced by a fresh one")

	// Verify the Secret was updated with the renewed token.
	updated := &corev1.Secret{}
	assert.NoError(t, r.Get(context.TODO(),
		types.NamespacedName{Name: getDexServerTokenSecretName(a), Namespace: a.Namespace},
		updated))
	assert.Equal(t, renewedToken, string(updated.Data["token"]))
}

func TestReconcileArgoCD_reconcileDexLegacySATokenSecrets(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex:      &argoproj.ArgoCDDexSpec{OpenShiftOAuth: true},
		}
	})

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, []client.Object{a}, []client.Object{a}, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// Create the Dex SA so the function can fetch it.
	_, err := r.reconcileServiceAccount(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, err)

	dexSAName := a.Name + "-" + common.ArgoCDDefaultDexServiceAccountName

	// Create a legacy kubernetes.io/service-account-token Secret.
	legacySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dexSAName + "-token-abc12",
			Namespace: a.Namespace,
			Labels: map[string]string{
				common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
			},
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: dexSAName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	assert.NoError(t, r.Create(context.TODO(), legacySecret))

	// Also add a reference to that Secret in the SA.secrets list.
	sa := &corev1.ServiceAccount{}
	assert.NoError(t, r.Get(context.TODO(),
		types.NamespacedName{Name: dexSAName, Namespace: a.Namespace}, sa))
	sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: legacySecret.Name})
	assert.NoError(t, r.Update(context.TODO(), sa))

	// Run the cleanup.
	assert.NoError(t, r.reconcileDexLegacySATokenSecrets(a))

	// Legacy Secret must be deleted.
	deleted := &corev1.Secret{}
	err = r.Get(context.TODO(),
		types.NamespacedName{Name: legacySecret.Name, Namespace: a.Namespace}, deleted)
	assert.True(t, apierrors.IsNotFound(err), "legacy SA token Secret must be deleted")

	// SA.secrets must no longer reference the legacy token.
	updatedSA := &corev1.ServiceAccount{}
	assert.NoError(t, r.Get(context.TODO(),
		types.NamespacedName{Name: dexSAName, Namespace: a.Namespace}, updatedSA))
	for _, ref := range updatedSA.Secrets {
		assert.False(t, strings.HasPrefix(ref.Name, dexSAName+"-token-"),
			"SA.secrets must not contain legacy token reference %q", ref.Name)
	}
}

func TestReconcileArgoCD_reconcileDexLegacySATokenSecrets_IgnoresUnrelatedSecrets(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(ac *argoproj.ArgoCD) {
		ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
			Dex:      &argoproj.ArgoCDDexSpec{OpenShiftOAuth: true},
		}
	})

	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, []client.Object{a}, []client.Object{a}, nil)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	_, err := r.reconcileServiceAccount(common.ArgoCDDefaultDexServiceAccountName, a)
	assert.NoError(t, err)

	// Opaque Secret with a similar name must not be deleted.
	opaqueSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-dex-server-token-opaque",
			Namespace: a.Namespace,
			Labels: map[string]string{
				common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
	assert.NoError(t, r.Create(context.TODO(), opaqueSecret))

	assert.NoError(t, r.reconcileDexLegacySATokenSecrets(a))

	// Opaque Secret must still exist.
	kept := &corev1.Secret{}
	assert.NoError(t, r.Get(context.TODO(),
		types.NamespacedName{Name: opaqueSecret.Name, Namespace: a.Namespace}, kept),
		"Opaque Secret must not be deleted by legacy cleanup")
}
