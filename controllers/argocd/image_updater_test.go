package argocd

import (
	"context"
	"fmt"
	"os"
	"slices"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
		// expandImageUpdaterWatchNamespaces matches directly against real Namespace
		// objects in the cluster.
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
	assert.True(t, *deployment.Spec.Template.Spec.AutomountServiceAccountToken)

	// No Command is expected: the container must inherit the image ENTRYPOINT so that
	// tini stays PID 1 and reaps git child processes (GITOPS-11648).
	want := []v1.Container{{
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

// TestReconcileImageUpdaterRoleForNamespace_BackfillsOwnerNamespaceLabel is a regression test for
// GITOPS-11825: reconcileRoleHelper only updates an existing Role when its policy rules differ, so
// a legacy Role (created before imageUpdaterOwnerNamespaceLabel existed) would otherwise never
// pick up the label and would remain dependent on its RoleBinding for ownership confirmation
// indefinitely. reconcileImageUpdaterRoleForNamespace must backfill the label on every reconcile
// of a still-desired namespace, even when the policy rules are already up to date.
func TestReconcileImageUpdaterRoleForNamespace_BackfillsOwnerNamespaceLabel(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	const targetNS = "target-ns"

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	desiredPolicyRules := policyRuleForRoleManagerRoleForImageUpdaterController()
	role, err := r.reconcileImageUpdaterRoleForNamespace(targetNS, a, desiredPolicyRules)
	require.NoError(t, err)
	require.NotNil(t, role)

	// Simulate a legacy Role created before imageUpdaterOwnerNamespaceLabel existed by stripping
	// the label, while leaving the policy rules unchanged.
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: targetNS}, role))
	delete(role.Labels, imageUpdaterOwnerNamespaceLabel)
	require.NoError(t, r.Update(context.TODO(), role))

	// Reconciling again with identical policy rules must still backfill the label: the rules
	// being unchanged must not short-circuit the backfill.
	_, err = r.reconcileImageUpdaterRoleForNamespace(targetNS, a, desiredPolicyRules)
	require.NoError(t, err)

	backfilled := &rbacv1.Role{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: targetNS}, backfilled))
	assert.Equal(t, a.Namespace, backfilled.Labels[imageUpdaterOwnerNamespaceLabel])
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
		// managedNamespaces, when non-empty, seeds r.ManagedNamespaces directly (bypassing
		// setManagedNamespaces, which this test does not exercise) to simulate namespaces
		// that already consented to being managed by this instance via the
		// "argocd.argoproj.io/managed-by" label or a NamespaceManagement CR.
		managedNamespaces []string
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
			clusterConfigNS:       testNamespace,
			clusterNamespaces:     []string{"ns1", "ns2", "other"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1", "ns2"},
		},
		{
			name:                  "exact list: single namespace",
			watchNamespacesEnv:    "ns1",
			clusterConfigNS:       testNamespace,
			clusterNamespaces:     []string{"ns1", "other"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1"},
		},
		{
			// IMAGE_UPDATER_WATCH_NAMESPACES="ns1,ns2" without the ArgoCD instance being in a
			// cluster-config namespace, and with neither namespace consenting: like "*", a
			// per-namespace watch list is normally an apps-in-any-namespace feature, but a
			// non-consenting entry is skipped leniently rather than failing the whole
			// reconciliation -- so no manager RBAC is granted anywhere.
			name:                  "exact list: namespaces skipped (not granted RBAC) when not a cluster-config namespace",
			watchNamespacesEnv:    "ns1,ns2",
			clusterNamespaces:     []string{"ns1", "ns2", "other"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{},
		},
		{
			// Every matched namespace already consented to being managed by this instance
			// (e.g. via the "argocd.argoproj.io/managed-by" label or a NamespaceManagement
			// CR), so the cluster-config-namespace restriction is waived: no new privilege
			// is being granted beyond what that consent already allows.
			name:                  "exact list: allowed when all matched namespaces already consent via managed-by",
			watchNamespacesEnv:    "ns1,ns2",
			clusterNamespaces:     []string{"ns1", "ns2", "other"},
			managedNamespaces:     []string{"ns1", "ns2"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1", "ns2"},
		},
		{
			// Only one of the two matched namespaces consented; the other would receive RBAC
			// it never agreed to, so it is skipped -- but ns1 still gets its manager RBAC
			// rather than the whole configuration being rejected.
			name:                  "exact list: only the consenting namespace gets RBAC when some matched namespaces don't consent",
			watchNamespacesEnv:    "ns1,ns2",
			clusterNamespaces:     []string{"ns1", "ns2", "other"},
			managedNamespaces:     []string{"ns1"},
			expectClusterRole:     false,
			expectManagerRoleInNS: []string{"ns1"},
		},
		{
			// A namespace listed that does not exist in the cluster must not receive any RBAC
			// objects; only namespaces that are also real cluster Namespaces are granted RBAC.
			name:                  "exact list: non-existent namespace gets no RBAC",
			watchNamespacesEnv:    "team-a,missing-ns",
			clusterConfigNS:       testNamespace,
			clusterNamespaces:     []string{"team-a"},
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
				resObjs = append(resObjs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name:   ns,
					Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
				}})
			}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			if len(tt.managedNamespaces) > 0 {
				r.ManagedNamespaces = &v1.NamespaceList{}
				for _, ns := range tt.managedNamespaces {
					r.ManagedNamespaces.Items = append(r.ManagedNamespaces.Items, v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
				}
			}

			sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

			ws, expanded := testResolveWatchNamespaces(t, r, a)
			_, err := r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
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

// TestPruneImageUpdaterNamespaceRBAC_ScopedByOwnerNamespace is a regression test for
// GITOPS-11825: two ArgoCD CRs that share the same name but live in different namespaces must
// not be able to prune each other's Image Updater Roles/RoleBindings. common.ArgoCDKeyName alone
// (cr.Name) is not a unique instance identifier, so pruning must also confirm ownership via the
// RoleBinding's ServiceAccount subject namespace, which is always set to the owning instance's
// namespace.
func TestPruneImageUpdaterNamespaceRBAC_ScopedByOwnerNamespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Two distinct ArgoCD instances with the same name, in different namespaces.
	instanceA := makeTestArgoCDInNamespace("instance-a", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	instanceB := makeTestArgoCDInNamespace("instance-b", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	require.Equal(t, instanceA.Name, instanceB.Name, "test requires both instances to share a name")

	resObjs := []client.Object{instanceA, instanceB}
	subresObjs := []client.Object{instanceA, instanceB}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	rules := policyRuleForRoleManagerRoleForImageUpdaterController()
	saA := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceA.Namespace}}
	saB := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceB.Namespace}}

	// Each instance watches its own distinct target namespace.
	roleA, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-a", instanceA, rules)
	require.NoError(t, err)
	require.NotNil(t, roleA)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-a", instanceA, roleA, saA))

	roleB, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-b", instanceB, rules)
	require.NoError(t, err)
	require.NotNil(t, roleB)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-b", instanceB, roleB, saB))

	// Pruning for instance A with an empty desired set (e.g. a rejected watch scope) must only
	// remove instance A's Role/RoleBinding in ns-for-a, leaving instance B's Role/RoleBinding in
	// ns-for-b untouched, even though both carry the same common.ArgoCDKeyName label value.
	require.NoError(t, r.pruneImageUpdaterNamespaceRBAC(instanceA, map[string]any{}))

	errA := r.Get(context.TODO(), types.NamespacedName{Name: roleA.Name, Namespace: "ns-for-a"}, &rbacv1.Role{})
	assert.True(t, errors.IsNotFound(errA), "instance A's role should have been pruned")

	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleB.Name, Namespace: "ns-for-b"}, &rbacv1.Role{}),
		"instance B's role must survive pruning triggered by instance A")
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceB.Name, "ns-for-b"),
		Namespace: "ns-for-b",
	}, &rbacv1.RoleBinding{}), "instance B's role binding must survive pruning triggered by instance A")
}

// TestPruneImageUpdaterNamespaceRBAC_RoleRetryableAfterRoleBindingDeleted is a regression test
// for GITOPS-11825: ownedRoleNames (derived from the RoleBinding list) is only populated while
// the matching RoleBinding still exists. If a previous reconcile deleted the RoleBinding but
// failed before deleting the Role it referenced (or before this call runs again), the stale
// Role must still be confirmed as owned and pruned via imageUpdaterOwnerNamespaceLabel, which is
// set on the Role itself and does not depend on the RoleBinding being present. Another
// same-named instance's Role in a different namespace must remain untouched.
func TestPruneImageUpdaterNamespaceRBAC_RoleRetryableAfterRoleBindingDeleted(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	instanceA := makeTestArgoCDInNamespace("instance-a", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	instanceB := makeTestArgoCDInNamespace("instance-b", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	require.Equal(t, instanceA.Name, instanceB.Name, "test requires both instances to share a name")

	resObjs := []client.Object{instanceA, instanceB}
	subresObjs := []client.Object{instanceA, instanceB}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	rules := policyRuleForRoleManagerRoleForImageUpdaterController()
	saA := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceA.Namespace}}
	saB := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceB.Namespace}}

	roleA, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-a", instanceA, rules)
	require.NoError(t, err)
	require.NotNil(t, roleA)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-a", instanceA, roleA, saA))

	roleB, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-b", instanceB, rules)
	require.NoError(t, err)
	require.NotNil(t, roleB)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-b", instanceB, roleB, saB))

	// Simulate a previous reconcile that deleted instance A's RoleBinding but failed/stopped
	// before the Role itself was pruned.
	rbA := &rbacv1.RoleBinding{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceA.Name, "ns-for-a"),
		Namespace: "ns-for-a",
	}, rbA))
	require.NoError(t, r.Delete(context.TODO(), rbA))

	// Retrying the prune with instance A's RoleBinding already gone must still remove the
	// now-orphaned Role, because it carries imageUpdaterOwnerNamespaceLabel = instanceA.Namespace.
	require.NoError(t, r.pruneImageUpdaterNamespaceRBAC(instanceA, map[string]any{}))

	errA := r.Get(context.TODO(), types.NamespacedName{Name: roleA.Name, Namespace: "ns-for-a"}, &rbacv1.Role{})
	assert.True(t, errors.IsNotFound(errA), "instance A's orphaned role should have been pruned on retry")

	// Instance B's Role/RoleBinding, in a different namespace, must survive.
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleB.Name, Namespace: "ns-for-b"}, &rbacv1.Role{}),
		"instance B's role must survive pruning triggered by instance A")
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceB.Name, "ns-for-b"),
		Namespace: "ns-for-b",
	}, &rbacv1.RoleBinding{}), "instance B's role binding must survive pruning triggered by instance A")
}

// TestPruneImageUpdaterNamespaceRBAC_LegacyRoleRetryAfterFailedDelete is a regression test for
// GITOPS-11825: a legacy Role (created before imageUpdaterOwnerNamespaceLabel existed) has no
// ownership evidence of its own -- it can only be confirmed owned via the RoleBinding that
// references it. If the Role is deleted before its RoleBinding and that Role deletion fails, the
// RoleBinding must survive so a retry can re-confirm ownership and try deleting the Role again.
// (Had the RoleBinding been deleted first, a retry could never again confirm ownership of the
// orphaned legacy Role, leaving it stuck forever.) Another same-named instance's Role/RoleBinding
// in a different namespace must remain untouched throughout.
func TestPruneImageUpdaterNamespaceRBAC_LegacyRoleRetryAfterFailedDelete(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	instanceA := makeTestArgoCDInNamespace("instance-a", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	instanceB := makeTestArgoCDInNamespace("instance-b", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	require.Equal(t, instanceA.Name, instanceB.Name, "test requires both instances to share a name")

	resObjs := []client.Object{instanceA, instanceB}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)

	deleteRoleErr := fmt.Errorf("simulated API error deleting role")
	roleDeleteCalls := 0
	cl := fake.NewClientBuilder().WithScheme(sch).WithObjects(resObjs...).WithStatusSubresource(resObjs...).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if _, ok := obj.(*rbacv1.Role); ok {
					roleDeleteCalls++
					if roleDeleteCalls == 1 {
						return deleteRoleErr
					}
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).Build()
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	rules := policyRuleForRoleManagerRoleForImageUpdaterController()
	saA := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceA.Namespace}}
	saB := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceB.Namespace}}

	roleA, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-a", instanceA, rules)
	require.NoError(t, err)
	require.NotNil(t, roleA)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-a", instanceA, roleA, saA))

	// Simulate a legacy Role created before imageUpdaterOwnerNamespaceLabel existed by stripping
	// the label: its only ownership evidence is now the RoleBinding created above.
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleA.Name, Namespace: "ns-for-a"}, roleA))
	delete(roleA.Labels, imageUpdaterOwnerNamespaceLabel)
	require.NoError(t, r.Update(context.TODO(), roleA))

	roleB, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-b", instanceB, rules)
	require.NoError(t, err)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-b", instanceB, roleB, saB))

	// First attempt: Role deletion fails. The RoleBinding must survive so ownership can be
	// re-confirmed on retry.
	err = r.pruneImageUpdaterNamespaceRBAC(instanceA, map[string]any{})
	require.ErrorIs(t, err, deleteRoleErr)

	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleA.Name, Namespace: "ns-for-a"}, &rbacv1.Role{}),
		"legacy role must survive a failed delete attempt")
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceA.Name, "ns-for-a"),
		Namespace: "ns-for-a",
	}, &rbacv1.RoleBinding{}), "role binding must survive so it can serve as ownership evidence on retry")

	// Retry: Role deletion now succeeds, so both the Role and its RoleBinding must be pruned.
	require.NoError(t, r.pruneImageUpdaterNamespaceRBAC(instanceA, map[string]any{}))

	errA := r.Get(context.TODO(), types.NamespacedName{Name: roleA.Name, Namespace: "ns-for-a"}, &rbacv1.Role{})
	assert.True(t, errors.IsNotFound(errA), "legacy role should have been pruned on retry")
	rbErrA := r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceA.Name, "ns-for-a"),
		Namespace: "ns-for-a",
	}, &rbacv1.RoleBinding{})
	assert.True(t, errors.IsNotFound(rbErrA), "role binding should have been pruned on retry")

	// Instance B's Role/RoleBinding, in a different namespace, must survive throughout.
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleB.Name, Namespace: "ns-for-b"}, &rbacv1.Role{}),
		"instance B's role must survive instance A's pruning")
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceB.Name, "ns-for-b"),
		Namespace: "ns-for-b",
	}, &rbacv1.RoleBinding{}), "instance B's role binding must survive instance A's pruning")
}

// TestReconcileImageUpdaterRBAC_PrunesStaleNamespaces verifies that when the
// watch-namespace list shrinks, reconcileImageUpdaterRBAC removes roles and bindings
// for the dropped namespaces without touching the remaining ones.
func TestReconcileImageUpdaterRBAC_PrunesStaleNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	// Per-namespace watch lists are an apps-in-any-namespace feature and require the
	// instance to live in a cluster-config namespace.
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)

	// Start with ns1 and ns2 in the watch list.
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "ns1,ns2"},
		}
	})

	resObjs := []client.Object{
		a,
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}}, // decoy: keeps patterns from matching the full cluster
	}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

	// First reconcile: both namespaces get roles.
	ws, expanded := testResolveWatchNamespaces(t, r, a)
	_, err := r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
	assert.NoError(t, err)

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
	_, err = r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
	assert.NoError(t, err)

	// ns1 still exists.
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces("ns1", a),
		Namespace: "ns1",
	}, &rbacv1.Role{}))

	// ns2 is gone.
	err = r.Get(context.TODO(), types.NamespacedName{
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

// TestReconcileImageUpdaterRBAC_RevokesStaleNamespacesWhenScopeRejected is a regression test:
// if a namespace-scoped instance already has per-namespace manager
// Roles/RoleBindings from an earlier reconcile (e.g. created before the cluster-scope
// restriction existed, or because the instance's cluster-config-namespace status changed),
// a later reconcile where none of those namespaces consent and the instance is no longer
// cluster-scoped must still revoke those existing grants (they are filtered out of the desired
// set) rather than leaving them in place indefinitely -- without failing the whole reconcile,
// since a non-consenting watch namespace should not be treated as a fatal error.
func TestReconcileImageUpdaterRBAC_RevokesStaleNamespacesWhenScopeRejected(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "ns1,ns2"},
		}
	})

	resObjs := []client.Object{
		a,
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns1", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns2", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace}}}, // decoy: keeps the pattern from matching every managed namespace
	}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}

	// First reconcile while cluster-scoped: both namespaces get manager Roles/RoleBindings.
	ws, expanded := testResolveWatchNamespaces(t, r, a)
	_, err := r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
	require.NoError(t, err)
	for _, ns := range []string{"ns1", "ns2"} {
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
			Namespace: ns,
		}, &rbacv1.Role{}), "expected role in %s after first reconcile", ns)
	}

	// The instance stops being cluster-scoped (e.g. ARGOCD_CLUSTER_CONFIG_NAMESPACES changes),
	// but the pattern-list env var is unchanged. Neither ns1 nor ns2 consents, so the next
	// reconcile must not grant them RBAC -- but, being lenient, it must not fail either...
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	ws, expanded = testResolveWatchNamespaces(t, r, a)
	_, err = r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
	require.NoError(t, err)

	// ...but must still revoke the previously-granted per-namespace RBAC rather than leaving
	// it in place indefinitely just because those namespaces no longer qualify.
	for _, ns := range []string{"ns1", "ns2"} {
		roleErr := r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
			Namespace: ns,
		}, &rbacv1.Role{})
		assert.True(t, errors.IsNotFound(roleErr), "stale role in %s should have been revoked, got: %v", ns, roleErr)

		rbErr := r.Get(context.TODO(), types.NamespacedName{
			Name:      getRoleBindingNameForSourceNamespaces(a.Name, ns),
			Namespace: ns,
		}, &rbacv1.RoleBinding{})
		assert.True(t, errors.IsNotFound(rbErr), "stale role binding in %s should have been revoked, got: %v", ns, rbErr)
	}
}

// TestReconcileImageUpdaterRBAC_RevokesClusterRBACWhenScopeRejected is a regression test
// if a cluster-scoped instance already created a ClusterRole/ClusterRoleBinding
// for IMAGE_UPDATER_WATCH_NAMESPACES="*", and later loses cluster-config eligibility (e.g.
// ARGOCD_CLUSTER_CONFIG_NAMESPACES changes) while "*" is still configured, the reconcile that
// rejects the now-invalid "*" config must still revoke the existing cluster-wide RBAC rather
// than leaving it in place indefinitely just because the reconcile errored.
func TestReconcileImageUpdaterRBAC_RevokesClusterRBACWhenScopeRejected(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "*"},
		}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}
	clusterRBACName := GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, a)

	// First reconcile while cluster-scoped: ClusterRole/ClusterRoleBinding get created.
	_, err := r.reconcileImageUpdaterRBAC(a, sa, "*", nil)
	require.NoError(t, err)
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRole{}))
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRoleBinding{}))

	// The instance stops being cluster-scoped, but "*" is unchanged. The next reconcile must
	// reject the config...
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	_, err = r.reconcileImageUpdaterRBAC(a, sa, "*", nil)
	require.Error(t, err)

	// ...but must still revoke the previously-granted cluster-wide RBAC.
	roleErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRole{})
	assert.True(t, errors.IsNotFound(roleErr), "stale cluster role should have been revoked, got: %v", roleErr)

	rbErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACName}, &rbacv1.ClusterRoleBinding{})
	assert.True(t, errors.IsNotFound(rbErr), "stale cluster role binding should have been revoked, got: %v", rbErr)
}

// TestReconcileImageUpdaterControllerEnabled_RevokesRBACOnInvalidWatchNamespaces is a regression
// test for GITOPS-11825: an invalid IMAGE_UPDATER_WATCH_NAMESPACES value (e.g. "*,team-a", which
// normalizeWatchNamespaces rejects) must not make reconcileImageUpdaterControllerEnabled return
// before revoking RBAC granted by a previous, valid configuration -- otherwise those grants
// remain active indefinitely across every subsequent reconcile. A same-named Argo CD instance in
// a different namespace must keep its own RBAC untouched by this cleanup.
func TestReconcileImageUpdaterControllerEnabled_RevokesRBACOnInvalidWatchNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "instance-a")

	instanceA := makeTestArgoCDInNamespace("instance-a", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	instanceB := makeTestArgoCDInNamespace("instance-b", func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
	})
	require.Equal(t, instanceA.Name, instanceB.Name, "test requires both instances to share a name")

	resObjs := []client.Object{instanceA, instanceB}
	subresObjs := []client.Object{instanceA, instanceB}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	saA := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceA.Namespace}}
	saB := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: instanceB.Namespace}}
	clusterRBACNameA := GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, instanceA)

	// Grant instance A cluster-wide RBAC under a previously valid "*" configuration, and grant
	// instance B its own per-namespace RBAC that must never be touched by instance A's cleanup.
	_, err := r.reconcileImageUpdaterRBAC(instanceA, saA, "*", nil)
	require.NoError(t, err)
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACNameA}, &rbacv1.ClusterRole{}))

	rules := policyRuleForRoleManagerRoleForImageUpdaterController()
	roleB, err := r.reconcileImageUpdaterRoleForNamespace("ns-for-b", instanceB, rules)
	require.NoError(t, err)
	require.NoError(t, r.reconcileImageUpdaterRoleBindingForNamespace("ns-for-b", instanceB, roleB, saB))

	// The env var now holds an ambiguous value that normalizeWatchNamespaces rejects.
	instanceA.Spec.ImageUpdater.Env = []v1.EnvVar{
		{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "*,team-a"},
	}

	err = r.reconcileImageUpdaterControllerEnabled(instanceA)
	require.Error(t, err)

	// Instance A's previously-granted cluster RBAC must be revoked, not left stale forever.
	roleErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACNameA}, &rbacv1.ClusterRole{})
	assert.True(t, errors.IsNotFound(roleErr), "instance A's stale cluster role should have been revoked, got: %v", roleErr)
	rbErr := r.Get(context.TODO(), types.NamespacedName{Name: clusterRBACNameA}, &rbacv1.ClusterRoleBinding{})
	assert.True(t, errors.IsNotFound(rbErr), "instance A's stale cluster role binding should have been revoked, got: %v", rbErr)

	// Instance B's RBAC, in a different namespace, must survive instance A's cleanup.
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: roleB.Name, Namespace: "ns-for-b"}, &rbacv1.Role{}),
		"instance B's role must survive instance A's cleanup")
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(instanceB.Name, "ns-for-b"),
		Namespace: "ns-for-b",
	}, &rbacv1.RoleBinding{}), "instance B's role binding must survive instance A's cleanup")
}

// TestReconcileImageUpdaterDeployment_WatchNamespacesExpanded verifies that the exact namespace
// names listed in IMAGE_UPDATER_WATCH_NAMESPACES are filtered down to those that exist in the
// cluster before being injected into the pod env.
func TestReconcileImageUpdaterDeployment_WatchNamespacesExpanded(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name            string
		clusterNS       []string
		watchNamespaces string
		wantEnvValue    string // expected value of IMAGE_UPDATER_WATCH_NAMESPACES in the pod, empty means env var should not be set
		wantEnvSet      bool   // whether IMAGE_UPDATER_WATCH_NAMESPACES should be present in the pod env
	}{
		{
			name:            "exact names are passed through unchanged",
			clusterNS:       []string{"ns1", "ns2", "other"},
			watchNamespaces: "ns1,ns2",
			wantEnvValue:    "ns1,ns2",
			wantEnvSet:      true,
		},
		{
			name:            "* is explicitly set to ensure cluster-scoped mode",
			watchNamespaces: "*",
			wantEnvValue:    "*",
			wantEnvSet:      true,
		},
		{
			name:            "empty string does not set IMAGE_UPDATER_WATCH_NAMESPACES (namespace-scoped mode uses default)",
			watchNamespaces: "",
			wantEnvSet:      false,
		},
		{
			// When the listed namespace does not exist in the cluster (or nothing consented),
			// the raw list must never be passed through: no RBAC was granted for it, so the pod
			// would end up watching a scope it has no permissions for.
			name:            "non-existent namespace is set to empty in the pod",
			clusterNS:       []string{"unrelated"},
			watchNamespaces: "app-ns",
			wantEnvValue:    "",
			wantEnvSet:      true,
		},
		{
			// Mixed list where one namespace exists and another does not.
			// expandImageUpdaterWatchNamespaces returns only existing namespaces, so
			// the non-existent "future-ns" is dropped from the deployment env var.
			// The concrete names are sorted.
			name:            "mixed-match: existing namespaces kept, non-existent namespace dropped",
			clusterNS:       []string{"team-a", "team-b", "unrelated"},
			watchNamespaces: "team-a,team-b,future-ns",
			wantEnvValue:    "team-a,team-b",
			wantEnvSet:      true,
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
				resObjs = append(resObjs, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name:   ns,
					Labels: map[string]string{common.ArgoCDManagedByLabel: a.Namespace},
				}})
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
			var found bool
			for _, e := range deployment.Spec.Template.Spec.Containers[0].Env {
				if e.Name == "IMAGE_UPDATER_WATCH_NAMESPACES" {
					gotValue = e.Value
					found = true
					break
				}
			}
			if tt.wantEnvSet {
				assert.True(t, found, "IMAGE_UPDATER_WATCH_NAMESPACES should be set")
				assert.Equal(t, tt.wantEnvValue, gotValue)
			} else {
				assert.False(t, found, "IMAGE_UPDATER_WATCH_NAMESPACES should not be set")
			}
		})
	}
}

// TestReconcileImageUpdaterControllerEnabled_DeploymentEnvMatchesGrantedRBAC verifies that the
// namespaces the image-updater pod actually watches (per IMAGE_UPDATER_WATCH_NAMESPACES in the
// Deployment, as interpreted by the image-updater binary) always have the manager RBAC it needs:
//   - env set to a list: every listed namespace has a manager Role + RoleBinding. In particular,
//     when every matched namespace is filtered out, the raw pattern must not be passed through.
//   - env unset: the binary watches only its own namespace, so the Role in cr.Namespace must
//     carry the manager rules too (base rules alone are not enough).
//
// It also verifies that namespaces that did not consent never receive RBAC.
func TestReconcileImageUpdaterControllerEnabled_DeploymentEnvMatchesGrantedRBAC(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name            string
		clusterConfig   bool // cr.Namespace is in ARGOCD_CLUSTER_CONFIG_NAMESPACES
		watchNamespaces string
		consented       []string // namespaces labelled managed-by=<cr.Namespace>
		wantNoRBACIn    []string // namespaces that must not receive a manager Role/RoleBinding
	}{
		{
			name:            "namespace-scoped: no matched namespace consented",
			watchNamespaces: "team-b",
			wantNoRBACIn:    []string{"team-b"},
		},
		{
			name:            "namespace-scoped: only some matched namespaces consented",
			watchNamespaces: "team-a,team-b",
			consented:       []string{"team-a"},
			wantNoRBACIn:    []string{"team-b"},
		},
		{
			name:            "namespace-scoped: own namespace listed alongside a non-consented one",
			watchNamespaces: testNamespace + ",team-b",
			wantNoRBACIn:    []string{"team-b"},
		},
		{
			// Listed namespace does not exist in the cluster (e.g. tenant not created yet).
			name:            "cluster-scoped: listed namespace does not exist",
			clusterConfig:   true,
			watchNamespaces: "future-ns",
		},
		{
			name:            "cluster-scoped: listed namespaces exist without consent",
			clusterConfig:   true,
			watchNamespaces: "team-a,team-b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clusterConfig {
				t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)
			} else {
				t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "")
			}
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ImageUpdater.Enabled = true
				a.Spec.ImageUpdater.Env = []v1.EnvVar{{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: tt.watchNamespaces}}
			})
			resObjs := []client.Object{a, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.Namespace}}}
			for _, ns := range []string{"team-a", "team-b"} {
				nsObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns, Labels: map[string]string{}}}
				if slices.Contains(tt.consented, ns) {
					nsObj.Labels[common.ArgoCDManagedByLabel] = a.Namespace
				}
				resObjs = append(resObjs, nsObj)
			}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, []client.Object{a}, []runtime.Object{})
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			require.NoError(t, r.setManagedNamespaces(a))

			require.NoError(t, r.reconcileImageUpdaterControllerEnabled(a))

			deployment := &appsv1.Deployment{}
			require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
				Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
				Namespace: a.Namespace,
			}, deployment))
			managerRules := policyRuleForRoleManagerRoleForImageUpdaterController()
			env := argoutil.EnvGet(deployment.Spec.Template.Spec.Containers[0].Env, "IMAGE_UPDATER_WATCH_NAMESPACES")
			if env == nil || env.Value == "" {
				// Unset or explicitly empty => the binary falls back to watching only its own
				// namespace, so the Role in cr.Namespace must grant the manager rules
				// (namespace-scoped "" mode), not just the base rules.
				role := &rbacv1.Role{}
				require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
					Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
					Namespace: a.Namespace,
				}, role))
				assert.Subset(t, role.Rules, managerRules,
					"IMAGE_UPDATER_WATCH_NAMESPACES is unset/empty, so the pod watches %q, but its Role there lacks the manager rules", a.Namespace)
			} else {
				for _, ns := range splitAndFilterPatterns(env.Value) {
					role := &rbacv1.Role{}
					err := r.Get(context.TODO(), types.NamespacedName{
						Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
						Namespace: ns,
					}, role)
					if !assert.NoError(t, err, "pod env IMAGE_UPDATER_WATCH_NAMESPACES=%q names namespace %q, which has no image-updater manager Role", env.Value, ns) {
						continue
					}
					assert.Equal(t, managerRules, role.Rules, "manager Role in %q has unexpected rules", ns)
					assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
						Name:      getRoleBindingNameForSourceNamespaces(a.Name, ns),
						Namespace: ns,
					}, &rbacv1.RoleBinding{}), "pod env names namespace %q, which has no image-updater manager RoleBinding", ns)
				}
			}

			// Namespaces that did not consent must never receive RBAC, whatever the env fallback is.
			for _, ns := range tt.wantNoRBACIn {
				err := r.Get(context.TODO(), types.NamespacedName{
					Name:      getRoleNameForApplicationSourceNamespaces(ns, a),
					Namespace: ns,
				}, &rbacv1.Role{})
				assert.True(t, errors.IsNotFound(err), "non-consenting namespace %q must not get a manager Role, got: %v", ns, err)
				err = r.Get(context.TODO(), types.NamespacedName{
					Name:      getRoleBindingNameForSourceNamespaces(a.Name, ns),
					Namespace: ns,
				}, &rbacv1.RoleBinding{})
				assert.True(t, errors.IsNotFound(err), "non-consenting namespace %q must not get a manager RoleBinding, got: %v", ns, err)
			}
		})
	}
}

func TestFilterImageUpdaterConsentedNamespaces(t *testing.T) {
	tests := []struct {
		name              string
		clusterConfig     bool     // whether cr.Namespace is a cluster-config namespace
		managedNamespaces []string // nil means r.ManagedNamespaces stays nil (unpopulated)
		namespaces        []string
		want              []string
	}{
		{name: "cluster-config namespace: every namespace is authorized regardless of consent", clusterConfig: true, namespaces: []string{"ns1", "ns2"}, want: []string{"ns1", "ns2"}},
		{name: "empty namespaces list stays empty", namespaces: nil, want: nil},
		{name: "empty namespaces list stays empty, even with populated ManagedNamespaces", managedNamespaces: []string{"ns1"}, namespaces: nil, want: nil},
		{name: "nil ManagedNamespaces excludes every namespace", managedNamespaces: nil, namespaces: []string{"ns1"}, want: nil},
		{name: "single namespace present in ManagedNamespaces is kept", managedNamespaces: []string{"ns1", "ns2"}, namespaces: []string{"ns1"}, want: []string{"ns1"}},
		{name: "all namespaces present in ManagedNamespaces are kept", managedNamespaces: []string{"ns1", "ns2", "ns3"}, namespaces: []string{"ns1", "ns2"}, want: []string{"ns1", "ns2"}},
		{name: "only the consenting namespace is kept when one is missing from ManagedNamespaces", managedNamespaces: []string{"ns1"}, namespaces: []string{"ns1", "ns2"}, want: []string{"ns1"}},
		{name: "no namespaces present in ManagedNamespaces excludes all", managedNamespaces: []string{"other"}, namespaces: []string{"ns1", "ns2"}, want: nil},
		{name: "empty ManagedNamespaces.Items excludes all", managedNamespaces: []string{}, namespaces: []string{"ns1"}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clusterConfig {
				t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)
			}
			r := &ReconcileArgoCD{}
			if tt.managedNamespaces != nil {
				r.ManagedNamespaces = &v1.NamespaceList{}
				for _, ns := range tt.managedNamespaces {
					r.ManagedNamespaces.Items = append(r.ManagedNamespaces.Items, v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
				}
			}
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {})
			assert.Equal(t, tt.want, r.filterImageUpdaterConsentedNamespaces(a, tt.namespaces))
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

		// * mixed with other namespace names is rejected.
		{name: "* mixed with a name", raw: "*,team-a", wantErr: true},
		{name: "name then *", raw: "team-a,*", wantErr: true},
		{name: "* in the middle of a list", raw: "app-ns,*,team-b", wantErr: true},

		// Valid namespace-name lists are returned unchanged.
		{name: "single namespace name", raw: "app-ns", want: "app-ns"},
		{name: "multiple namespace names", raw: "app-ns,team-b", want: "app-ns,team-b"},
		{name: "name with trailing comma (raw unchanged)", raw: "app-ns,", want: "app-ns,"},

		// Lists naming only the CR's own namespace are not a privilege escalation,
		// so they collapse to the namespace-scoped sentinel.
		{name: "exact match of own namespace", raw: testNamespace, want: ""},
		{name: "own namespace repeated", raw: testNamespace + "," + testNamespace, want: ""},
		{name: "own namespace with trailing comma", raw: testNamespace + ",", want: ""},
		{name: "own namespace alongside another namespace is not collapsed", raw: testNamespace + ",team-a", want: testNamespace + ",team-a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeWatchNamespaces(tt.raw, testNamespace)
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
		terminatingNS   []string // Namespaces to mark as terminating (DeletionTimestamp != nil)
		watchNamespaces string   // raw value passed to expandImageUpdaterWatchNamespaces
		want            []string // expected result, must be sorted
		wantErr         bool     // true when an error is expected instead of a result
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
			name:            "whitespace trimmed from each name",
			clusterNS:       []string{"ns1", "ns2", "unrelated"},
			watchNamespaces: " ns1 , ns2 ",
			want:            []string{"ns1", "ns2"},
		},
		{
			name:            "result is sorted regardless of cluster order",
			clusterNS:       []string{"z-ns", "a-ns", "m-ns", "not-a-match"},
			watchNamespaces: "z-ns,a-ns,m-ns",
			want:            []string{"a-ns", "m-ns", "z-ns"},
		},
		{
			name:            "namespace listed twice is returned only once",
			clusterNS:       []string{"ns1", "other"},
			watchNamespaces: "ns1,ns1",
			want:            []string{"ns1"},
		},
		{
			name:            "name matches no namespaces",
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
			clusterNS:       []string{"app-ns", "other-ns"},
			watchNamespaces: ",app-ns,",
			want:            []string{"app-ns"},
		},
		{
			name:            "only commas returns nil (no valid patterns)",
			clusterNS:       []string{"app-ns"},
			watchNamespaces: ",,,",
			want:            nil,
		},
		// Listing every active cluster namespace by exact name is legitimate: the comma-list
		// mode is itself gated by reconcileImageUpdaterRBAC requiring cr.Namespace to be a
		// cluster-config namespace before it is allowed at all. Unlike "*", listing every
		// namespace here does not bypass that restriction.
		{
			// cr.Namespace (testNamespace, "argocd") is a real Namespace object in the fake
			// client (see the test loop below) and is deliberately not excluded from matching,
			// so listing it by name picks it up too.
			name:            "listing every cluster namespace by name matches all of them, including cr.Namespace",
			clusterNS:       []string{"ns1", "ns2", "ns3"},
			watchNamespaces: testNamespace + ",ns1,ns2,ns3",
			want:            []string{testNamespace, "ns1", "ns2", "ns3"},
		},
		{
			name:            "name list that matches a strict subset is allowed",
			clusterNS:       []string{"app-ns-1", "app-ns-2", "other-ns"},
			watchNamespaces: "app-ns-1,app-ns-2",
			want:            []string{"app-ns-1", "app-ns-2"},
		},
		{
			name:            "terminating namespace is excluded from matches",
			clusterNS:       []string{"ns1", "ns2-terminating", "other"},
			terminatingNS:   []string{"ns2-terminating"},
			watchNamespaces: "ns1,ns2-terminating",
			want:            []string{"ns1"},
		},
		{
			name:            "terminating namespace does not prevent valid name matches",
			clusterNS:       []string{"app-a", "app-b", "app-term", "other-ns"},
			terminatingNS:   []string{"app-term"},
			watchNamespaces: "app-a,app-b,app-term",
			want:            []string{"app-a", "app-b"},
		},
		{
			name:            "name list matching all active cluster namespaces (with terminating ones present) is allowed",
			clusterNS:       []string{"ns1", "ns2", "ns-term"},
			terminatingNS:   []string{"ns-term"},
			watchNamespaces: "ns1,ns2,ns-term",
			want:            []string{"ns1", "ns2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.ImageUpdater.Enabled = true
			})

			// cr.Namespace is a real Namespace in any actual cluster, so it must exist as a
			// real object here too for expandImageUpdaterWatchNamespaces (which now lists
			// every Namespace in the cluster) to be able to match it.
			resObjs := []client.Object{a, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.Namespace}}}
			terminatingSet := make(map[string]struct{})
			for _, ns := range tt.terminatingNS {
				terminatingSet[ns] = struct{}{}
			}
			for _, ns := range tt.clusterNS {
				nsObj := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				if _, isTerminating := terminatingSet[ns]; isTerminating {
					// Mark namespace as terminating by setting DeletionTimestamp and Finalizers
					// (fake client requires finalizers when DeletionTimestamp is set)
					now := metav1.Now()
					nsObj.DeletionTimestamp = &now
					nsObj.Finalizers = []string{"kubernetes"}
				}
				resObjs = append(resObjs, nsObj)
			}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, resObjs, []runtime.Object{})
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

			got, err := r.expandImageUpdaterWatchNamespaces(tt.watchNamespaces)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestReconcileImageUpdaterRBAC_PreservesCRNamespaceInWatchList is a regression test for
// GITOPS-11825: when an authorized (cluster-scoped) instance in namespace A explicitly lists
// its own namespace alongside another ("A,B"), expansion must not silently drop A. Dropping it
// would make the deployed controller watch only B, while A would receive just the base Role
// (configmaps/secrets/leases/events) and never the manager Role (applications/imageupdaters),
// leaving the controller unable to manage Applications in its own namespace despite the admin
// explicitly asking for it.
func TestReconcileImageUpdaterRBAC_PreservesCRNamespaceInWatchList(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ImageUpdater.Enabled = true
		a.Spec.ImageUpdater.Env = []v1.EnvVar{
			// testNamespace (cr.Namespace) is explicitly listed alongside team-b.
			{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: testNamespace + ",team-b"},
		}
	})

	resObjs := []client.Object{
		a,
		// cr.Namespace is a real Namespace in any actual cluster, so it must exist as a real
		// object here too for expandImageUpdaterWatchNamespaces to be able to match it.
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: a.Namespace}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-b"}},
	}
	subresObjs := []client.Object{a}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme, promoter.AddToScheme, apiregistrationv1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, []runtime.Object{})
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	ws, expanded := testResolveWatchNamespaces(t, r, a)
	assert.Equal(t, []string{testNamespace, "team-b"}, expanded, "cr.Namespace must be preserved when explicitly listed, not silently dropped")

	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa", Namespace: a.Namespace}}
	_, err := r.reconcileImageUpdaterRBAC(a, sa, ws, expanded)
	require.NoError(t, err)

	// cr.Namespace must receive the manager Role/RoleBinding in addition to the base Role,
	// so the controller can actually manage Applications/ImageUpdaters in its own namespace.
	managerRole := &rbacv1.Role{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces(testNamespace, a),
		Namespace: testNamespace,
	}, managerRole), "expected a manager role for cr.Namespace")
	assert.Equal(t, policyRuleForRoleManagerRoleForImageUpdaterController(), managerRole.Rules)

	managerRB := &rbacv1.RoleBinding{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleBindingNameForSourceNamespaces(a.Name, testNamespace),
		Namespace: testNamespace,
	}, managerRB), "expected a manager role binding for cr.Namespace")
	assert.Equal(t, managerRole.Name, managerRB.RoleRef.Name)

	// team-b gets the same treatment.
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      getRoleNameForApplicationSourceNamespaces("team-b", a),
		Namespace: "team-b",
	}, &rbacv1.Role{}))

	// cr.Namespace must still also have the base role, unaffected.
	baseRole := &rbacv1.Role{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: testNamespace,
	}, baseRole))
	assert.Equal(t, policyRuleForRoleForImageUpdaterController(), baseRole.Rules)

	// Deployment env var must preserve cr.Namespace in the watch list too.
	assert.NoError(t, r.reconcileImageUpdaterDeployment(a, sa, ws, expanded))
	deployment := &appsv1.Deployment{}
	require.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDImageUpdaterControllerComponent, a),
		Namespace: a.Namespace,
	}, deployment))
	env := argoutil.EnvGet(deployment.Spec.Template.Spec.Containers[0].Env, "IMAGE_UPDATER_WATCH_NAMESPACES")
	require.NotNil(t, env)
	assert.Equal(t, testNamespace+",team-b", env.Value)
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
