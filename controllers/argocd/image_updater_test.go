package argocd

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	tlsProfile "github.com/argoproj-labs/argocd-operator/pkg/tlsprofile"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclient "k8s.io/client-go/kubernetes/fake"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// testResolveWatchNamespaces mirrors the logic in reconcileImageUpdaterControllerEnabled:
// it extracts watchNamespaces from the CR env and expands patterns to concrete namespace
// names using the fake client. Tests use this to obtain pre-computed values before calling
// reconcileImageUpdaterRBAC or reconcileImageUpdaterDeployment directly.
func testResolveWatchNamespaces(t *testing.T, r *ReconcileArgoCD, a *argoproj.ArgoCD) (string, []string) {
	t.Helper()
	watchNamespaces := ""
	if env := argoutil.EnvGet(a.Spec.ImageUpdater.Env, "IMAGE_UPDATER_WATCH_NAMESPACES"); env != nil {
		watchNamespaces = strings.TrimSpace(env.Value)
	}
	var expanded []string
	if watchNamespaces != "" && watchNamespaces != "*" {
		var err error
		expanded, err = r.expandImageUpdaterWatchNamespaces(watchNamespaces)
		require.NoError(t, err)
	}
	return watchNamespaces, expanded
}

func TestReconcileImageUpdater_CreateRoles(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa, "", nil))

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
					Optional: new(true),
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
					Optional: new(true),
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
					Optional: new(true),
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
					Optional:   new(true),
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
	err := r.reconcileImageUpdaterDeployment(a, &sa, "", nil)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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

func TestReconcileImageUpdaterRBAC_WatchScope(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name string
		// watchNamespacesEnv is the raw value placed in IMAGE_UPDATER_WATCH_NAMESPACES.
		// An empty string means the env var is not set at all.
		watchNamespacesEnv string
		// clusterConfigNS, when non-empty, is set as ARGOCD_CLUSTER_CONFIG_NAMESPACES.
		clusterConfigNS string
		// clusterNamespaces are Namespace objects pre-created in the fake client so that
		// expandImageUpdaterWatchNamespaces can match patterns against real namespace names.
		clusterNamespaces []string
		expectError       bool
		expectClusterRole bool
		expectClusterRB   bool
		// expectManagerRoleInNS lists every namespace where a manager Role is expected.
		// Use testNamespace to assert the combined (base + manager) role in cr.Namespace.
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
			name:                  "exact list: two namespaces",
			watchNamespacesEnv:    "ns1,ns2",
			clusterNamespaces:     []string{"ns1", "ns2"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1", "ns2"},
		},
		{
			name:                  "exact list: single namespace",
			watchNamespacesEnv:    "ns1",
			clusterNamespaces:     []string{"ns1"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1"},
		},
		{
			// Wildcard suffix — the typical Apps-in-Any-Namespace tenant pattern.
			name:                  "glob wildcard: suffix pattern matches subset of namespaces",
			watchNamespacesEnv:    "*-argocd",
			clusterNamespaces:     []string{"team-a-argocd", "team-b-argocd", "unrelated"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"team-a-argocd", "team-b-argocd"},
		},
		{
			// Multiple patterns — one glob and one exact name.
			name:                  "glob wildcard: multiple patterns",
			watchNamespacesEnv:    "*-argocd,staging",
			clusterNamespaces:     []string{"team-a-argocd", "staging", "prod"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"team-a-argocd", "staging"},
		},
		{
			// A namespace that exists in the cluster but is not matched by the pattern must
			// not receive any RBAC objects.
			name:                  "glob wildcard: non-matching namespaces get no RBAC",
			watchNamespacesEnv:    "team-*",
			clusterNamespaces:     []string{"team-a", "other"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"team-a"},
		},
		{
			name:                  "cluster-scoped: watch namespaces set to *",
			watchNamespacesEnv:    "*",
			clusterConfigNS:       testNamespace,
			expectClusterRole:     true,
			expectClusterRB:       true,
			expectManagerRoleInNS: []string{},
		},
		{
			// IMAGE_UPDATER_WATCH_NAMESPACES="*" without the ArgoCD instance being in a
			// cluster-config namespace must be rejected to prevent privilege escalation.
			name:               "cluster-scoped: * rejected when not a cluster-config namespace",
			watchNamespacesEnv: "*",
			expectError:        true,
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
			for _, ns := range tt.clusterNamespaces {
				resObjs = append(resObjs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
			}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

			ws, expanded := testResolveWatchNamespaces(t, r, a)
			err := r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
			if tt.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			for _, ns := range tt.expectManagerRoleInNS {
				if ns == testNamespace {
					// Namespace-scoped mode: base + manager rules merged into a single role in cr.Namespace.
					role := &rbacv1.Role{}
					assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
						Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
						Namespace: ns,
					}, role), "expected combined role in namespace %s", ns)
					assert.NotEmpty(t, role.Rules)
				} else {
					// Pattern-list mode: per-namespace manager role.
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

			// Assert namespaces that exist in the cluster but were NOT matched get no RBAC.
			allMatched := make(map[string]struct{}, len(tt.expectManagerRoleInNS))
			for _, ns := range tt.expectManagerRoleInNS {
				allMatched[ns] = struct{}{}
			}
			for _, ns := range tt.clusterNamespaces {
				if _, expected := allMatched[ns]; expected {
					continue
				}
				err := r.Get(context.TODO(), types.NamespacedName{
					Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
					Namespace: ns,
				}, &rbacv1.Role{})
				assert.True(t, errors.IsNotFound(err), "namespace %s should not have a manager role, got error: %v", ns, err)
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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}
	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa, "", nil))

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
	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, &sa, "", nil))

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
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
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
		desired := map[string]any{"ns1": nil}
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
		assert.NoError(t, r.pruneImageUpdaterNamespaceRBAC(a, map[string]any{}))

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
		assert.NoError(t, r.pruneImageUpdaterNamespaceRBAC(a, map[string]any{}))
	})
}

// TestReconcileImageUpdaterRBAC_PrunesStaleNamespaces verifies that when the
// watch-namespace list shrinks, reconcileImageUpdaterRBAC removes roles and bindings
// for the dropped namespaces without touching the remaining ones.
func TestReconcileImageUpdaterRBAC_PrunesStaleNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Start with ns1 and ns2 in the watch list.
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "ns1,ns2"},
		}
	})

	resObjs := []client.Object{
		a,
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
	}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

	// First reconcile: both namespaces get roles.
	ws, expanded := testResolveWatchNamespaces(t, r, a)
	assert.NoError(t, r.reconcileImageUpdaterRBAC(a, sa, ws, expanded))

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
	ws, expanded = testResolveWatchNamespaces(t, r, a)
	assert.NoError(t, r.reconcileImageUpdaterRBAC(a, sa, ws, expanded))

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

// TestReconcileImageUpdaterDeployment_WatchNamespacesExpanded verifies that glob/regex patterns
// in IMAGE_UPDATER_WATCH_NAMESPACES are resolved to concrete namespace names before being
// injected into the pod env, because the image-updater controller itself does not understand patterns.
func TestReconcileImageUpdaterDeployment_WatchNamespacesExpanded(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name            string
		clusterNS       []string
		watchNamespaces string
		wantEnvValue    string // expected value of IMAGE_UPDATER_WATCH_NAMESPACES in the pod
	}{
		{
			name:            "glob pattern is expanded to concrete names",
			clusterNS:       []string{"team-a-argocd", "team-b-argocd", "unrelated"},
			watchNamespaces: "*-argocd",
			wantEnvValue:    "team-a-argocd,team-b-argocd",
		},
		{
			name:            "exact names are passed through unchanged",
			clusterNS:       []string{"ns1", "ns2"},
			watchNamespaces: "ns1,ns2",
			wantEnvValue:    "ns1,ns2",
		},
		{
			name:            "* is passed through unchanged (cluster-scoped mode)",
			watchNamespaces: "*",
			wantEnvValue:    "*",
		},
		{
			// When the pattern matches no existing namespaces the env var must NOT be
			// replaced with "". Overwriting with "" would silently put the pod into
			// namespace-scoped mode while the RBAC for that mode was never created.
			name:            "no-match pattern is left unchanged in the pod",
			clusterNS:       []string{"unrelated"},
			watchNamespaces: "app-*",
			wantEnvValue:    "app-*",
		},
		{
			// Mixed list where one pattern has matches and another does not.
			// expandImageUpdaterWatchNamespaces returns only concrete matches, so
			// the unmatched "future-*" is dropped from the deployment env var.
			// The concrete names are sorted.
			name:            "mixed-match: matched patterns expanded, unmatched pattern dropped",
			clusterNS:       []string{"team-a", "team-b", "unrelated"},
			watchNamespaces: "team-*,future-*",
			wantEnvValue:    "team-a,team-b",
		},
		{
			// Exact name alongside an unmatched glob: exact name expands normally,
			// unmatched glob is dropped from the env var.
			name:            "exact name expands, unmatched glob dropped",
			clusterNS:       []string{"ns1"},
			watchNamespaces: "ns1,app-*",
			wantEnvValue:    "ns1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ImageUpdater.Enabled = true
				a.Spec.ImageUpdater.Env = []v1.EnvVar{
					{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: tt.watchNamespaces},
				}
			})

			resObjs := []client.Object{a}
			for _, ns := range tt.clusterNS {
				resObjs = append(resObjs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
			}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, resObjs, []runtime.Object{})
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}
			ws, expanded := testResolveWatchNamespaces(t, r, a)
			assert.NoError(t, r.reconcileImageUpdaterDeployment(a, sa, ws, expanded))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
				Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
				Namespace: a.Namespace,
			}, deployment))

			var gotValue string
			for _, e := range deployment.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "IMAGE_UPDATER_WATCH_NAMESPACES" {
					gotValue = e.Value
					break
				}
			}
			assert.Equal(t, tt.wantEnvValue, gotValue)
		})
	}
}

func TestNormalizeWatchNamespaces(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		// Canonical sentinels pass through unchanged.
		{name: "empty string (namespace-scoped)", raw: "", want: ""},
		{name: "bare * (cluster-scoped)", raw: "*", want: "*"},

		// Trailing/leading commas around sole *.
		{name: "trailing comma on *", raw: "*,", want: "*"},
		{name: "leading comma on *", raw: ",*", want: "*"},
		{name: "* surrounded by commas", raw: ",*,", want: "*"},

		// Only commas collapse to namespace-scoped.
		{name: "only commas", raw: ",,,", want: ""},

		// * mixed with other patterns is rejected.
		{name: "* mixed with pattern", raw: "*,team-a", wantErr: true},
		{name: "pattern then *", raw: "team-a,*", wantErr: true},
		{name: "* in the middle of a list", raw: "app-*,*,team-b", wantErr: true},

		// Valid pattern lists are returned unchanged.
		{name: "single glob pattern", raw: "app-*", want: "app-*"},
		{name: "multiple patterns", raw: "app-*,team-b", want: "app-*,team-b"},
		{name: "regex pattern", raw: "/^app-[a-z]+$/", want: "/^app-[a-z]+$/"},
		{name: "pattern with trailing comma (raw unchanged)", raw: "app-*,", want: "app-*,"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWatchNamespaces(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExpandImageUpdaterWatchNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name            string
		clusterNS       []string // Namespace objects to pre-create in the fake client
		watchNamespaces string   // raw value passed to expandImageUpdaterWatchNamespaces
		want            []string // expected result, must be sorted
	}{
		{
			name:            "exact match: single namespace",
			clusterNS:       []string{"ns1", "ns2"},
			watchNamespaces: "ns1",
			want:            []string{"ns1"},
		},
		{
			name:            "exact match: multiple namespaces",
			clusterNS:       []string{"ns1", "ns2", "ns3"},
			watchNamespaces: "ns1,ns2",
			want:            []string{"ns1", "ns2"},
		},
		{
			name:            "glob: suffix wildcard",
			clusterNS:       []string{"team-a-argocd", "team-b-argocd", "unrelated"},
			watchNamespaces: "*-argocd",
			want:            []string{"team-a-argocd", "team-b-argocd"},
		},
		{
			name:            "glob: prefix wildcard",
			clusterNS:       []string{"argocd-east", "argocd-west", "other"},
			watchNamespaces: "argocd-*",
			want:            []string{"argocd-east", "argocd-west"},
		},
		{
			name:            "glob: multiple patterns, one glob and one exact",
			clusterNS:       []string{"team-a-argocd", "staging", "prod"},
			watchNamespaces: "*-argocd,staging",
			want:            []string{"staging", "team-a-argocd"},
		},
		{
			// Regex patterns must be wrapped in "/" to be treated as a true regular expression;
			// without the slashes the pattern falls through to glob matching.
			name:            "regex: slash-delimited pattern matches numeric suffix",
			clusterNS:       []string{"tenant-001", "tenant-002", "tenant-abc", "not-tenant"},
			watchNamespaces: "/tenant-[0-9]+/",
			want:            []string{"tenant-001", "tenant-002"},
		},
		{
			// Glob character classes work without regex delimiters.
			name:            "glob: character class in pattern",
			clusterNS:       []string{"tenant-001", "tenant-002", "tenant-abc"},
			watchNamespaces: "tenant-[0-9][0-9][0-9]",
			want:            []string{"tenant-001", "tenant-002"},
		},
		{
			name:            "whitespace trimmed from each pattern",
			clusterNS:       []string{"ns1", "ns2"},
			watchNamespaces: " ns1 , ns2 ",
			want:            []string{"ns1", "ns2"},
		},
		{
			name:            "result is sorted regardless of cluster order",
			clusterNS:       []string{"z-ns", "a-ns", "m-ns"},
			watchNamespaces: "*-ns",
			want:            []string{"a-ns", "m-ns", "z-ns"},
		},
		{
			name:            "namespace matched by two patterns is returned only once",
			clusterNS:       []string{"ns1"},
			watchNamespaces: "ns1,ns*",
			want:            []string{"ns1"},
		},
		{
			name:            "pattern matches no namespaces",
			clusterNS:       []string{"ns1", "ns2"},
			watchNamespaces: "nonexistent",
			want:            nil,
		},
		{
			name:            "no cluster namespaces",
			clusterNS:       []string{},
			watchNamespaces: "ns1",
			want:            nil,
		},
		{
			name:            "trailing comma does not produce an empty match-all pattern",
			clusterNS:       []string{"app-ns", "other-ns"},
			watchNamespaces: "app-ns,",
			want:            []string{"app-ns"},
		},
		{
			name:            "leading and trailing commas are stripped",
			clusterNS:       []string{"app-ns"},
			watchNamespaces: ",app-ns,",
			want:            []string{"app-ns"},
		},
		{
			name:            "only commas returns nil (no valid patterns)",
			clusterNS:       []string{"app-ns"},
			watchNamespaces: ",,,",
			want:            nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ImageUpdater.Enabled = true
			})

			resObjs := []client.Object{a}
			for _, ns := range tt.clusterNS {
				resObjs = append(resObjs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
			}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, resObjs, []runtime.Object{})
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			got, err := r.expandImageUpdaterWatchNamespaces(tt.watchNamespaces)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReconcileImageUpdaterDeployment_TLSArgs(t *testing.T) {
	tests := []struct {
		name         string
		centralTLS   tlsProfile.TLSConfigProfile
		expectedArgs []string
	}{
		{
			name: "central tls profile",
			centralTLS: tlsProfile.TLSConfigProfile{
				DisableClusterTLSProfile: false,
				MinVersion:               configv1.VersionTLS12,
				Ciphers: []string{
					"ECDHE-RSA-AES128-GCM-SHA256",
					"ECDHE-RSA-AES256-GCM-SHA384",
				},
			},
			expectedArgs: []string{
				"--tlsminversion",
				"1.2",
				"--tlsciphers",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256:TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			},
		},
		{
			name: "Disable cluster tls profile",
			centralTLS: tlsProfile.TLSConfigProfile{
				DisableClusterTLSProfile: true,
			},
			expectedArgs: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()

			_ = appsv1.AddToScheme(scheme)
			_ = v1.AddToScheme(scheme)
			_ = argoproj.AddToScheme(scheme)
			cr := makeTestArgoCD()
			cr.Spec.ImageUpdater.Enabled = true
			sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "test-sa", Namespace: cr.Namespace}}
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, sa).Build()
			r := &ReconcileArgoCD{
				Client:                  client,
				Scheme:                  scheme,
				CentralTLSConfigProfile: tt.centralTLS,
			}
			err := r.reconcileImageUpdaterDeployment(cr, sa, "", nil)
			require.NoError(t, err)
			deployment := &appsv1.Deployment{}
			err = client.Get(context.TODO(), types.NamespacedName{Name: nameWithSuffix(common.ArgoCDImageUpdaterControllerComponent, cr), Namespace: cr.Namespace}, deployment)
			require.NoError(t, err)
			args := deployment.Spec.Template.Spec.Containers[0].Args
			for _, expected := range tt.expectedArgs {
				assert.Contains(t, args, expected)
			}
		})
	}
}
