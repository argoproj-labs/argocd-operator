package argocd

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	"github.com/stretchr/testify/assert"
	testclient "k8s.io/client-go/kubernetes/fake"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileImageUpdater_CreateRoles(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	desiredPolicyRules := policyRuleForRoleForImageUpdaterController()

	_, err := r.reconcileImageUpdaterRole(a, desiredPolicyRules)
	assert.NoError(t, err)

	testRole := &rbacv1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, testRole))

	assert.Equal(t, desiredPolicyRules, testRole.Rules)

	a.Spec.ImageUpdater.Enabled = false
	_, err = r.reconcileImageUpdaterRole(a, desiredPolicyRules)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, testRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_CreateClusterRoles(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	_, err := r.reconcileImageUpdaterClusterRole(a)
	assert.NoError(t, err)

	testRole := &rbacv1.ClusterRole{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
	}, testRole))

	desiredPolicyRules := policyRuleForRoleManagerRoleForImageUpdaterController()

	assert.Equal(t, desiredPolicyRules, testRole.Rules)

	a.Spec.ImageUpdater.Enabled = false
	_, err = r.reconcileImageUpdaterClusterRole(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
	}, testRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_CreateServiceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	desiredSa, err := r.reconcileImageUpdaterServiceAccount(a)
	assert.NoError(t, err)

	testSa := &v1.ServiceAccount{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, testSa))

	assert.Equal(t, testSa.Name, desiredSa.Name)

	a.Spec.ImageUpdater.Enabled = false
	_, err = r.reconcileImageUpdaterServiceAccount(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, testSa)
	assert.True(t, errors.IsNotFound(err))

}

func TestReconcileImageUpdater_CreateRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileImageUpdaterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
			Namespace: a.Namespace,
		},
		roleBinding))

	assert.Equal(t, roleBinding.RoleRef.Name, role.Name)
	assert.Equal(t, roleBinding.Subjects[0].Name, sa.Name)

	a.Spec.ImageUpdater.Enabled = false
	err = r.reconcileImageUpdaterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_CreateClusterRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	role := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "cluster-role-name"}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileImageUpdaterClusterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.ClusterRoleBinding{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name: GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		},
		roleBinding))

	assert.Equal(t, roleBinding.RoleRef.Name, role.Name)
	assert.Equal(t, roleBinding.Subjects[0].Name, sa.Name)

	a.Spec.ImageUpdater.Enabled = false
	err = r.reconcileImageUpdaterClusterRoleBinding(a, role, sa)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_CreateDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.Name)

	want := []v1.Container{{
		Command:         []string{"/manager"},
		Args:            []string{"run"},
		Image:           argoutil.CombineImageTag(DefaultImageUpdaterImage, DefaultImageUpdaterTag),
		ImagePullPolicy: v1.PullIfNotPresent,
		Name:            common.ArgoCDImageUpdaterControllerComponent,
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "image-updater-conf",
				MountPath: "/app/config",
			},
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			},
			{
				Name:      "ssh-config",
				MountPath: "/app/.ssh",
			},
			{
				Name:      "tmp",
				MountPath: "/tmp",
			},
			{
				Name:      "ssh-signing-key",
				MountPath: "/app/ssh-keys/id_rsa",
				ReadOnly:  true,
				SubPath:   "sshPrivateKey",
			},
		},
		Resources: v1.ResourceRequirements{},
		LivenessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{
						IntVal: int32(8081),
					},
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
		},
		ReadinessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				HTTPGet: &v1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{
						IntVal: int32(8081),
					},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile image-updater-controller deployment containers:\n%s", diff)
	}

	volumes := []v1.Volume{
		{
			Name: "tls-certs",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "argocd-tls-certs-cm",
					},
				},
			},
		},
		{
			Name: "image-updater-conf",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: v1.LocalObjectReference{
						Name: ArgocdImageUpdaterConfigCM,
					},
					Items: []v1.KeyToPath{
						{
							Key:  "registries.conf",
							Path: "registries.conf",
						},
						{
							Key:  "git.commit-message-template",
							Path: "commit.template",
						},
					},
				},
			},
		},
		{
			Name: "ssh-known-hosts",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: v1.LocalObjectReference{
						Name: "argocd-ssh-known-hosts-cm",
					},
				},
			},
		},
		{
			Name: "ssh-config",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: v1.LocalObjectReference{
						Name: ArgocdImageUpdaterSSHConfigCM,
					},
				},
			},
		},
		{
			Name: "ssh-signing-key",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: "ssh-git-creds",
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile image-updater-controller deployment volumes:\n%s", diff)
	}

	expectedSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: deployment.Name,
		},
	}

	if diff := cmp.Diff(expectedSelector, deployment.Spec.Selector); diff != "" {
		t.Fatalf("failed to reconcile image-updater-controller label selector:\n%s", diff)
	}

	a.Spec.ImageUpdater.Enabled = false
	err := r.reconcileImageUpdaterDeployment(a, &sa)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_CreateSecret(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileImageUpdaterSecret(a)
	assert.NoError(t, err)

	testSecret := &v1.Secret{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-image-updater-secret",
		Namespace: a.Namespace,
	}, testSecret))

	a.Spec.ImageUpdater.Enabled = false
	err = r.reconcileImageUpdaterSecret(a)
	assert.NoError(t, err)
	secret := &v1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-image-updater-secret", Namespace: a.Namespace}, secret)
	assertNotFound(t, err)
}

func TestReconcileImageUpdater_CreateConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	imageUpdaterConfigMaps := []*v1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ArgocdImageUpdaterConfigCM,
				Namespace: a.Namespace,
			},
		},
	}

	err := r.reconcileImageUpdaterConfigMap(a, imageUpdaterConfigMaps[0])
	assert.NoError(t, err)

	testConfigMap := &v1.ConfigMap{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-image-updater-config",
		Namespace: a.Namespace,
	}, testConfigMap))

	a.Spec.ImageUpdater.Enabled = false
	err = r.reconcileImageUpdaterConfigMap(a, testConfigMap)
	assert.NoError(t, err)
	configMap := &v1.ConfigMap{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-image-updater-config", Namespace: a.Namespace}, configMap)
	assertNotFound(t, err)
}

func TestDeleteImageUpdaterClusterRBAC(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	clusterRBACName := GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a)

	t.Run("no-op when ClusterRole and ClusterRoleBinding do not exist", func(t *testing.T) {
		assert.NoError(t, r.deleteImageUpdaterClusterRBAC(a))
	})

	t.Run("deletes existing ClusterRole and ClusterRoleBinding", func(t *testing.T) {
		// Pre-create the ClusterRole and ClusterRoleBinding the same way the enabled reconciler would.
		clusterRole, err := r.reconcileImageUpdaterClusterRole(a)
		assert.NoError(t, err)
		assert.NotNil(t, clusterRole)

		if clusterRole != nil {
			assert.NoError(t, r.reconcileImageUpdaterClusterRoleBinding(a, clusterRole, &v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace},
			}))
		}

		// Verify they exist before deletion.
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRole{}))
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRoleBinding{}))

		// Delete.
		assert.NoError(t, r.deleteImageUpdaterClusterRBAC(a))

		// Verify they are gone.
		err = r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRole{})
		assert.True(t, errors.IsNotFound(err))

		err = r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRoleBinding{})
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("idempotent: second call is a no-op after deletion", func(t *testing.T) {
		assert.NoError(t, r.deleteImageUpdaterClusterRBAC(a))
	})
}

func TestReconcileImageUpdater_RoleForNamespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	const targetNS = "target-ns"

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	desiredPolicyRules := policyRuleForRoleManagerRoleForImageUpdaterController()
	_, err := r.reconcileImageUpdaterRoleForNamespace(targetNS, a, desiredPolicyRules)
	assert.NoError(t, err)

	testRole := &rbacv1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces(targetNS, a),
		Namespace: targetNS,
	}, testRole))

	assert.Equal(t, targetNS, testRole.Namespace)
	assert.Equal(t, desiredPolicyRules, testRole.Rules)

	a.Spec.ImageUpdater.Enabled = false
	_, err = r.reconcileImageUpdaterRoleForNamespace(targetNS, a, desiredPolicyRules)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces(targetNS, a),
		Namespace: targetNS,
	}, testRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_RoleBindingForNamespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	const targetNS = "target-ns"

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
		Name:      getRoleNameForApplicationSourceNamespaces(targetNS, a),
		Namespace: targetNS,
	}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      "sa-name",
		Namespace: a.Namespace,
	}}

	err := r.reconcileImageUpdaterRoleBindingForNamespace(targetNS, a, role, sa)
	assert.NoError(t, err)

	rb := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(a.Name, targetNS),
		Namespace: targetNS,
	}, rb))

	// RoleBinding must be in targetNS, not cr.Namespace
	assert.Equal(t, targetNS, rb.Namespace)
	assert.Equal(t, role.Name, rb.RoleRef.Name)
	assert.Equal(t, sa.Name, rb.Subjects[0].Name)
	// Subject namespace must be explicit because SA is in a different namespace
	assert.Equal(t, a.Namespace, rb.Subjects[0].Namespace)

	a.Spec.ImageUpdater.Enabled = false
	err = r.reconcileImageUpdaterRoleBindingForNamespace(targetNS, a, role, sa)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(a.Name, targetNS),
		Namespace: targetNS,
	}, rb)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileImageUpdater_WatchNamespacesMode(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name               string
		watchNamespacesEnv string // raw value set in the env var; empty string means env var not set
		// clusterConfigNS, when non-empty, is set as ARGOCD_CLUSTER_CONFIG_NAMESPACES for the subtest
		clusterConfigNS   string
		expectClusterRole bool
		expectClusterRB   bool
		// namespaces where a manager role is expected (testNamespace = combined role in own ns)
		expectManagerRoleInNS []string
	}{
		{
			name:                  "namespace-scoped: env var not set",
			watchNamespacesEnv:    "",
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{testNamespace},
		},
		{
			name:                  "namespace-scoped: env var set to whitespace",
			watchNamespacesEnv:    "  ",
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{testNamespace},
		},
		{
			name:                  "comma-separated: two namespaces",
			watchNamespacesEnv:    "ns1,ns2",
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1", "ns2"},
		},
		{
			name:                  "comma-separated: single namespace",
			watchNamespacesEnv:    "ns1",
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1"},
		},
		{
			name:                  "cluster-scoped: watch namespaces set to *",
			watchNamespacesEnv:    "*",
			clusterConfigNS:       testNamespace,
			expectClusterRole:     true,
			expectClusterRB:       true,
			expectManagerRoleInNS: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clusterConfigNS != "" {
				t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", tt.clusterConfigNS)
			}

			envVars := []v1.EnvVar{}
			if tt.watchNamespacesEnv != "" {
				envVars = append(envVars, v1.EnvVar{
					Name:  "IMAGE_UPDATER_WATCH_NAMESPACES",
					Value: tt.watchNamespacesEnv,
				})
			}

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ImageUpdater.Enabled = true
				a.Spec.ImageUpdater.Env = envVars
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			assert.NoError(t, r.reconcileImageUpdaterControllerEnabled(a))

			for _, ns := range tt.expectManagerRoleInNS {
				if ns == testNamespace {
					// Namespace-scoped: base + manager rules are merged into a single role.
					role := &rbacv1.Role{}
					assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
						Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
						Namespace: ns,
					}, role), "expected combined role in namespace %s", ns)
					assert.NotEmpty(t, role.Rules)
				} else {
					// Comma-separated: manager role in the listed namespace.
					role := &rbacv1.Role{}
					assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
						Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
						Namespace: ns,
					}, role), "expected manager role in namespace %s", ns)
					assert.Equal(t, policyRuleForRoleManagerRoleForImageUpdaterController(), role.Rules)

					rb := &rbacv1.RoleBinding{}
					assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
						Name:      getRoleBindingNameForSourceNamespaces(a.Name, ns),
						Namespace: ns,
					}, rb), "expected manager role binding in namespace %s", ns)
					assert.Equal(t, role.Name, rb.RoleRef.Name)
					assert.Equal(t, a.Namespace, rb.Subjects[0].Namespace)
				}
			}

			clusterRBACName := GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a)

			clusterRoleErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRole{})
			if tt.expectClusterRole {
				assert.NoError(t, clusterRoleErr, "expected ClusterRole to exist")
			} else {
				assert.True(t, errors.IsNotFound(clusterRoleErr), "expected ClusterRole to be absent")
			}

			clusterRBErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRoleBinding{})
			if tt.expectClusterRB {
				assert.NoError(t, clusterRBErr, "expected ClusterRoleBinding to exist")
			} else {
				assert.True(t, errors.IsNotFound(clusterRBErr), "expected ClusterRoleBinding to be absent")
			}
		})
	}
}

func TestReconcileImageUpdater_testEnvVars(t *testing.T) {

	envMap := []v1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = envMap
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}
	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(envMap, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("failed to reconcile image-updater-controller deployment env:\n%s", diff)
	}

	// Verify any manual updates to the env vars should be overridden by the operator.
	unwantedEnv := []v1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
		{
			Name:  "ping",
			Value: "pong",
		},
	}

	deployment.Spec.Template.Spec.Containers[0].Env = unwantedEnv
	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile back
	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(envMap, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("operator failed to override the manual changes to image updater controller:\n%s", diff)
	}
}

func TestPruneImageUpdaterNamespaceRBAC(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	rules := policyRuleForRoleManagerRoleForImageUpdaterController()
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

	// Create roles and bindings in ns1, ns2, ns3.
	for _, ns := range []string{"ns1", "ns2", "ns3"} {
		role, err := r.reconcileImageUpdaterRoleForNamespace(ns, a, rules)
		assert.NoError(t, err)
		assert.NotNil(t, role)
		assert.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace(ns, a, role, sa))
	}

	// Verify label is present on the created role.
	roleNs1 := &rbacv1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces("ns1", a),
		Namespace: "ns1",
	}, roleNs1))
	assert.Equal(t, "true", roleNs1.Labels[imageUpdaterManagedNamespaceLabel])

	t.Run("prune removes roles not in the desired set", func(t *testing.T) {
		// Keep only ns1; ns2 and ns3 should be pruned.
		desired := map[string]struct{}{"ns1": {}}
		assert.NoError(t, r.pruneImageUpdaterNamespaceRBAC(a, desired))

		// ns1 must still exist.
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleNameForApplicationSourceNamespaces("ns1", a),
			Namespace: "ns1",
		}, &rbacv1.Role{}))
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleBindingNameForSourceNamespaces(a.Name, "ns1"),
			Namespace: "ns1",
		}, &rbacv1.RoleBinding{}))

		// ns2 and ns3 must be gone.
		for _, ns := range []string{"ns2", "ns3"} {
			err := r.Get(context.TODO(), types.NamespacedName{
				Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
				Namespace: ns,
			}, &rbacv1.Role{})
			assert.True(t, errors.IsNotFound(err), "expected role in %s to be deleted", ns)

			err = r.Get(context.TODO(), types.NamespacedName{
				Name:      getRoleBindingNameForSourceNamespaces(a.Name, ns),
				Namespace: ns,
			}, &rbacv1.RoleBinding{})
			assert.True(t, errors.IsNotFound(err), "expected role binding in %s to be deleted", ns)
		}
	})

	t.Run("prune with empty set removes all remaining namespace RBAC", func(t *testing.T) {
		assert.NoError(t, r.pruneImageUpdaterNamespaceRBAC(a, map[string]struct{}{}))

		err := r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleNameForApplicationSourceNamespaces("ns1", a),
			Namespace: "ns1",
		}, &rbacv1.Role{})
		assert.True(t, errors.IsNotFound(err))

		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleBindingNameForSourceNamespaces(a.Name, "ns1"),
			Namespace: "ns1",
		}, &rbacv1.RoleBinding{})
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("prune is idempotent on empty cluster", func(t *testing.T) {
		assert.NoError(t, r.pruneImageUpdaterNamespaceRBAC(a, map[string]struct{}{}))
	})
}

// TestReconcileImageUpdaterControllerEnabled_PrunesStaleNamespaceRBAC verifies that when the
// watch-namespace list shrinks, the operator removes roles and bindings for the dropped namespaces
// without touching the remaining ones.
func TestReconcileImageUpdaterControllerEnabled_PrunesStaleNamespaceRBAC(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Start with ns1 and ns2 in the watch list.
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "ns1,ns2"},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// First reconcile: both namespaces get roles.
	assert.NoError(t, r.reconcileImageUpdaterControllerEnabled(a))

	for _, ns := range []string{"ns1", "ns2"} {
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
			Namespace: ns,
		}, &rbacv1.Role{}), "expected role in %s after first reconcile", ns)
	}

	// Shrink to ns1 only.
	a.Spec.ImageUpdater.Env = []v1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "ns1"},
	}

	// Second reconcile: ns2 role and binding should be pruned.
	assert.NoError(t, r.reconcileImageUpdaterControllerEnabled(a))

	// ns1 still exists.
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces("ns1", a),
		Namespace: "ns1",
	}, &rbacv1.Role{}))

	// ns2 is gone.
	err := r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces("ns2", a),
		Namespace: "ns2",
	}, &rbacv1.Role{})
	assert.True(t, errors.IsNotFound(err), "stale role in ns2 should have been pruned")

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(a.Name, "ns2"),
		Namespace: "ns2",
	}, &rbacv1.RoleBinding{})
	assert.True(t, errors.IsNotFound(err), "stale role binding in ns2 should have been pruned")
}
