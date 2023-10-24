package argocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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
		{
			name:       "dex disabled by specifying different provider",
			setEnvFunc: nil,
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "true")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)
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
		{
			name:       "dex disabled by switching provider",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			// ensure deployment was created correctly
			deployment := &appsv1.Deployment{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))
			deployment = &appsv1.Deployment{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, deployment)

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
			r := makeTestReconciler(cl, sch)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexDeployment(test.argoCD))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Client.Get(
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
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileDexDeployment(a))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
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
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
					RunAsNonRoot: boolPtr(true),
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "static-files",
						MountPath: "/shared",
					},
				},
				ImagePullPolicy: corev1.PullAlways,
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
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
					RunAsNonRoot: boolPtr(true),
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "static-files", MountPath: "/shared"}},
			},
		},
		ServiceAccountName: "argocd-argocd-dex-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}
	assert.Equal(t, want, deployment.Spec.Template.Spec)
}

func TestReconcileArgoCD_reconcileDexDeployment_withUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	desiredPodSpec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "static-files",
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
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
					RunAsNonRoot: boolPtr(true),
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "static-files",
						MountPath: "/shared",
					},
				},
				ImagePullPolicy: corev1.PullAlways,
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
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
					RunAsNonRoot: boolPtr(true),
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "static-files", MountPath: "/shared"}},
			},
		},
		ServiceAccountName: "argocd-argocd-dex-server",
		NodeSelector:       common.DefaultNodeSelector(),
	}

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
			wantPodSpec: desiredPodSpec,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

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
			assert.NoError(t, r.Client.Get(
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
		{
			name:       "dex disabled by switching provider",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileDexService(test.argoCD))

			// ensure service was created correctly
			service := &corev1.Service{}
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, service)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			assert.NoError(t, r.reconcileDexService(test.argoCD))
			service = &corev1.Service{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-dex-server", Namespace: test.argoCD.Namespace}, service)

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
		{
			name:       "dex disabled by switching provider",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			sa, err := r.reconcileServiceAccount(common.ArgoCDDexServerComponent, test.argoCD)
			assert.NoError(t, err)

			// ensure serviceaccount was created correctly
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: test.argoCD.Namespace}, sa)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			_, err = r.reconcileServiceAccount(common.ArgoCDDexServerComponent, test.argoCD)
			assert.NoError(t, err)

			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: sa.Name, Namespace: test.argoCD.Namespace}, sa)

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
		{
			name:       "dex disabled by switching provider",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			rules := policyRuleForDexServer()
			role := newRole(common.ArgoCDDexServerComponent, rules, test.argoCD)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			_, err := r.reconcileRole(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			// ensure role was created correctly
			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: test.argoCD.Namespace}, role)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			_, err = r.reconcileRole(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: test.argoCD.Namespace}, role)

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
		{
			name:       "dex disabled by switching provider",
			setEnvFunc: nil,
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
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
			r := makeTestReconciler(cl, sch)

			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			rules := policyRuleForDexServer()
			roleBinding := newRoleBindingWithname(common.ArgoCDDexServerComponent, test.argoCD)

			if test.setEnvFunc != nil {
				test.setEnvFunc(t, "false")
			}

			assert.NoError(t, r.reconcileRoleBinding(common.ArgoCDDexServerComponent, rules, test.argoCD))
			assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding))

			// ensure roleBinding was created correctly
			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding)
			assert.NoError(t, err)

			if test.updateEnvFunc != nil {
				test.updateEnvFunc(t, "true")
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			err = r.reconcileRoleBinding(common.ArgoCDDexServerComponent, rules, test.argoCD)
			assert.NoError(t, err)

			err = r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: test.argoCD.Namespace}, roleBinding)

			if test.wantRoleBindingDeleted {
				assertNotFound(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
