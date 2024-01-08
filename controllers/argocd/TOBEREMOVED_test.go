package argocd

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

func setRouteAPIFound(t *testing.T, routeEnabled bool) {
	routeAPIEnabledTemp := routeAPIFound
	t.Cleanup(func() {
		routeAPIFound = routeAPIEnabledTemp
	})
	routeAPIFound = routeEnabled
}

func makeTestReconciler(client client.Client, sch *runtime.Scheme) *ReconcileArgoCD {
	return &ReconcileArgoCD{
		Client: client,
		Scheme: sch,
	}
}

func createNamespace(r *ReconcileArgoCD, n string, managedBy string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	if managedBy != "" {
		ns.Labels = map[string]string{common.ArgoCDManagedByLabel: managedBy}
	}

	if r.ManagedNamespaces == nil {
		r.ManagedNamespaces = &corev1.NamespaceList{}
	}
	r.ManagedNamespaces.Items = append(r.ManagedNamespaces.Items, *ns)

	return r.Client.Create(context.TODO(), ns)
}

func createNamespaceManagedByClusterArgoCDLabel(r *ReconcileArgoCD, n string, managedBy string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	if managedBy != "" {
		ns.Labels = map[string]string{common.ArgoCDManagedByClusterArgoCDLabel: managedBy}
	}

	if r.ManagedSourceNamespaces == nil {
		r.ManagedSourceNamespaces = make(map[string]string)
	}
	r.ManagedSourceNamespaces[ns.Name] = ""

	return r.Client.Create(context.TODO(), ns)
}

func stringMapKeys(m map[string]string) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}

func merge(base map[string]string, diff map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range base {
		result[k] = v
	}
	for k, v := range diff {
		result[k] = v
	}

	return result
}

func TestReconcileNotifications_CreateRoles(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	_, err := r.reconcileNotificationsRole(a)
	assert.NoError(t, err)

	testRole := &rbacv1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testRole))

	desiredPolicyRules := policyRuleForNotificationsController()

	assert.Equal(t, desiredPolicyRules, testRole.Rules)

	a.Spec.Notifications.Enabled = false
	_, err = r.reconcileNotificationsRole(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_CreateServiceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	desiredSa, err := r.reconcileNotificationsServiceAccount(a)
	assert.NoError(t, err)

	testSa := &corev1.ServiceAccount{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testSa))

	assert.Equal(t, testSa.Name, desiredSa.Name)

	a.Spec.Notifications.Enabled = false
	_, err = r.reconcileNotificationsServiceAccount(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testSa)
	assert.True(t, errors.IsNotFound(err))

}

func TestReconcileNotifications_CreateRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileNotificationsRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
			Namespace: a.Namespace,
		},
		roleBinding))

	assert.Equal(t, roleBinding.RoleRef.Name, role.Name)
	assert.Equal(t, roleBinding.Subjects[0].Name, sa.Name)

	a.Spec.Notifications.Enabled = false
	err = r.reconcileNotificationsRoleBinding(a, role, sa)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_CreateDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)
	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)

	want := []corev1.Container{{
		Command:         []string{"argocd-notifications", "--loglevel", "info"},
		Image:           argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-notifications-controller",
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
		Resources:  corev1.ResourceRequirements{},
		WorkingDir: "/app",
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(9001),
					},
				},
			},
		},
	}}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller deployment containers:\n%s", diff)
	}

	volumes := []corev1.Volume{
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-tls-certs-cm",
					},
				},
			},
		},
		{
			Name: "argocd-repo-server-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "argocd-repo-server-tls",
					Optional:   boolPtr(true),
				},
			},
		},
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller deployment volumes:\n%s", diff)
	}

	expectedSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: deployment.Name,
		},
	}

	if diff := cmp.Diff(expectedSelector, deployment.Spec.Selector); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller label selector:\n%s", diff)
	}

	a.Spec.Notifications.Enabled = false
	err := r.reconcileNotificationsDeployment(a, &sa)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_CreateSecret(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileNotificationsSecret(a)
	assert.NoError(t, err)

	testSecret := &corev1.Secret{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-notifications-secret",
		Namespace: a.Namespace,
	}, testSecret))

	a.Spec.Notifications.Enabled = false
	err = r.reconcileNotificationsSecret(a)
	assert.NoError(t, err)
	secret := &corev1.Secret{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-notifications-secret", Namespace: a.Namespace}, secret)
	assertNotFound(t, err)
}

func TestReconcileNotifications_CreateConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileNotificationsConfigMap(a)
	assert.NoError(t, err)

	testCm := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-notifications-cm",
		Namespace: a.Namespace,
	}, testCm))

	assert.True(t, len(testCm.Data) > 0)

	a.Spec.Notifications.Enabled = false
	err = r.reconcileNotificationsConfigMap(a)
	assert.NoError(t, err)
	testCm = &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-notifications-cm", Namespace: a.Namespace}, testCm)
	assertNotFound(t, err)
}

func TestReconcileNotifications_testEnvVars(t *testing.T) {

	envMap := []corev1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
		a.Spec.Notifications.Env = envMap
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(envMap, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller deployment env:\n%s", diff)
	}

	// Verify any manual updates to the env vars should be overridden by the operator.
	unwantedEnv := []corev1.EnvVar{
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
	assert.NoError(t, r.Client.Update(context.TODO(), deployment))

	// Reconcile back
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(envMap, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("operator failed to override the manual changes to notification controller:\n%s", diff)
	}
}

func TestReconcileNotifications_testLogLevel(t *testing.T) {

	testLogLevel := "debug"
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
		a.Spec.Notifications.LogLevel = testLogLevel
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	expectedCMD := []string{
		"argocd-notifications",
		"--loglevel",
		"debug",
	}

	if diff := cmp.Diff(expectedCMD, deployment.Spec.Template.Spec.Containers[0].Command); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller deployment logLevel:\n%s", diff)
	}

	// Verify any manual updates to the logLevel should be overridden by the operator.
	unwantedCommand := []string{
		"argocd-notifications",
		"--logLevel",
		"info",
	}

	deployment.Spec.Template.Spec.Containers[0].Command = unwantedCommand
	assert.NoError(t, r.Client.Update(context.TODO(), deployment))

	// Reconcile back
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(expectedCMD, deployment.Spec.Template.Spec.Containers[0].Command); diff != "" {
		t.Fatalf("operator failed to override the manual changes to notification controller:\n%s", diff)
	}
}

func applicationSetDefaultVolumeMounts() []corev1.VolumeMount {
	repoMounts := repoServerDefaultVolumeMounts()
	ignoredMounts := map[string]bool{
		"plugins":                             true,
		"argocd-repo-server-tls":              true,
		common.ArgoCDRedisServerTLSSecretName: true,
	}
	mounts := make([]corev1.VolumeMount, len(repoMounts)-len(ignoredMounts), len(repoMounts)-len(ignoredMounts))
	j := 0
	for _, mount := range repoMounts {
		if !ignoredMounts[mount.Name] {
			mounts[j] = mount
			j += 1
		}
	}
	return mounts
}

func applicationSetDefaultVolumes() []corev1.Volume {
	repoVolumes := repoServerDefaultVolumes()
	ignoredVolumes := map[string]bool{
		"var-files":                           true,
		"plugins":                             true,
		"argocd-repo-server-tls":              true,
		common.ArgoCDRedisServerTLSSecretName: true,
	}
	volumes := make([]corev1.Volume, len(repoVolumes)-len(ignoredVolumes), len(repoVolumes)-len(ignoredVolumes))
	j := 0
	for _, volume := range repoVolumes {
		if !ignoredVolumes[volume.Name] {
			volumes[j] = volume
			j += 1
		}
	}
	return volumes
}

func TestReconcileApplicationSet_CreateDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, a)
}

func checkExpectedDeploymentValues(t *testing.T, r *ReconcileArgoCD, deployment *appsv1.Deployment, sa *corev1.ServiceAccount, a *argoproj.ArgoCD) {
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	want := []corev1.Container{applicationSetContainer(a, false)}

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

	volumes := []corev1.Volume{
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDKnownHostsConfigMapName,
					},
				},
			},
		},
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keys",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDGPGKeysConfigMapName,
					},
				},
			},
		},
		{
			Name: "gpg-keyring",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if a.Spec.ApplicationSet.SCMRootCAConfigMap != "" && argoutil.IsObjectFound(r.Client, a.Namespace, common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName, a) {
		volumes = append(volumes, corev1.Volume{
			Name: "appset-gitlab-scm-tls-cert",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
					},
				},
			},
		})
	}

	if diff := cmp.Diff(volumes, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment volumes:\n%s", diff)
	}

	expectedSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: deployment.Name,
		},
	}

	if diff := cmp.Diff(expectedSelector, deployment.Spec.Selector); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller label selector:\n%s", diff)
	}
}

func TestReconcileApplicationSetProxyConfiguration(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Proxy Env vars
	setProxyEnvVars(t)

	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	r.reconcileApplicationSetDeployment(a, &sa)

	want := []corev1.EnvVar{
		{
			Name:  "HTTPS_PROXY",
			Value: "https://example.com",
		},
		{
			Name:  "HTTP_PROXY",
			Value: "http://example.com",
		},
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name:  "NO_PROXY",
			Value: ".cluster.local",
		},
	}

	deployment := &appsv1.Deployment{}

	// reconcile ApplicationSets
	r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment)

	if diff := cmp.Diff(want, deployment.Spec.Template.Spec.Containers[0].Env); diff != "" {
		t.Fatalf("failed to reconcile applicationset-controller deployment containers:\n%s", diff)
	}

}

func TestReconcileApplicationSet_UpdateExistingDeployments(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	existingDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name + "-applicationset-controller",
			Namespace: a.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "fake-container",
						},
					},
				},
			},
		},
	}

	resObjs := []client.Object{a, existingDeployment}
	subresObjs := []client.Object{a, existingDeployment}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the updated Deployment has the expected properties
	checkExpectedDeploymentValues(t, r, deployment, &sa, a)

}

func TestReconcileApplicationSet_Deployments_resourceRequirements(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDWithResources()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sa := corev1.ServiceAccount{}

	assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.ObjectMeta.Name)
	appsetAssertExpectedLabels(t, &deployment.ObjectMeta)

	containerWant := []corev1.Container{applicationSetContainer(a, false)}

	if diff := cmp.Diff(containerWant, deployment.Spec.Template.Spec.Containers); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}

	volumesWant := applicationSetDefaultVolumes()

	if diff := cmp.Diff(volumesWant, deployment.Spec.Template.Spec.Volumes); diff != "" {
		t.Fatalf("failed to reconcile argocd-server deployment:\n%s", diff)
	}
}

func TestReconcileApplicationSet_Deployments_SpecOverride(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                   string
		appSetField            *argoproj.ArgoCDApplicationSet
		envVars                map[string]string
		expectedContainerImage string
	}{
		{
			name:                   "unspecified fields should use default",
			appSetField:            &argoproj.ArgoCDApplicationSet{},
			expectedContainerImage: argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
		},
		{
			name: "ensure that sha hashes are formatted correctly",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
			},
			expectedContainerImage: "custom-image@sha256:b835999eb5cf75d01a2678cd971095926d9c2566c9ffe746d04b83a6a0a2849f",
		},
		{
			name: "custom image should properly substitute",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name:                   "verify env var substitution overrides default",
			appSetField:            &argoproj.ArgoCDApplicationSet{},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-env-image",
		},

		{
			name: "env var should not override spec fields",
			appSetField: &argoproj.ArgoCDApplicationSet{
				Image:   "custom-image",
				Version: "custom-version",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-image:custom-version",
		},
		{
			name: "ensure scm tls cert mount is present",
			appSetField: &argoproj.ArgoCDApplicationSet{
				SCMRootCAConfigMap: "test-scm-tls-mount",
			},
			envVars:                map[string]string{common.ArgoCDImageEnvName: "custom-env-image"},
			expectedContainerImage: "custom-env-image",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			for testEnvName, testEnvValue := range test.envVars {
				t.Setenv(testEnvName, testEnvValue)
			}

			a := makeTestArgoCD()
			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			r.Client.Create(context.Background(), cm, &client.CreateOptions{})

			a.Spec.ApplicationSet = test.appSetField

			sa := corev1.ServiceAccount{}
			assert.NoError(t, r.reconcileApplicationSetDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-applicationset-controller",
					Namespace: a.Namespace,
				},
				deployment))

			specImage := deployment.Spec.Template.Spec.Containers[0].Image
			assert.Equal(t, test.expectedContainerImage, specImage)
			checkExpectedDeploymentValues(t, r, deployment, &sa, a)
		})
	}

}

func TestReconcileApplicationSet_ServiceAccount(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	retSa, err := r.reconcileApplicationSetServiceAccount(a)
	assert.NoError(t, err)

	sa := &corev1.ServiceAccount{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		sa))

	assert.Equal(t, sa.Name, retSa.Name)

	appsetAssertExpectedLabels(t, &sa.ObjectMeta)
}

func TestReconcileApplicationSet_Role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	roleRet, err := r.reconcileApplicationSetRole(a)
	assert.NoError(t, err)

	role := &rbacv1.Role{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		role))

	assert.Equal(t, roleRet.Name, role.Name)
	appsetAssertExpectedLabels(t, &role.ObjectMeta)

	expectedResources := []string{
		"deployments",
		"secrets",
		"configmaps",
		"events",
		"applicationsets/status",
		"applications",
		"applicationsets",
		"appprojects",
		"applicationsets/finalizers",
	}

	foundResources := []string{}

	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			foundResources = append(foundResources, resource)
		}
	}

	sort.Strings(expectedResources)
	sort.Strings(foundResources)

	assert.Equal(t, expectedResources, foundResources)
}

func TestReconcileApplicationSet_RoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileApplicationSetRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		roleBinding))

	appsetAssertExpectedLabels(t, &roleBinding.ObjectMeta)

	assert.Equal(t, roleBinding.RoleRef.Name, role.Name)
	assert.Equal(t, roleBinding.Subjects[0].Name, sa.Name)

}

func appsetAssertExpectedLabels(t *testing.T, meta *metav1.ObjectMeta) {
	assert.Equal(t, meta.Labels["app.kubernetes.io/name"], "argocd-applicationset-controller")
	assert.Equal(t, meta.Labels["app.kubernetes.io/part-of"], "argocd-applicationset")
	assert.Equal(t, meta.Labels["app.kubernetes.io/component"], "controller")
}

func setProxyEnvVars(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "https://example.com")
	t.Setenv("HTTP_PROXY", "http://example.com")
	t.Setenv("NO_PROXY", ".cluster.local")
}

func TestReconcileApplicationSet_Service(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	s := newServiceWithSuffix(common.ApplicationSetServiceNameSuffix, common.ApplicationSetServiceNameSuffix, a)

	assert.NoError(t, r.reconcileApplicationSetService(a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Namespace: s.Namespace, Name: s.Name}, s))
}

func TestArgoCDApplicationSetCommand(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	baseCommand := []string{
		"entrypoint.sh",
		"argocd-applicationset-controller",
		"--argocd-repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
		"--loglevel",
		"info",
	}

	// When a single command argument is passed
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--foo",
		"bar",
	}

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	cmd := append(baseCommand, "--foo", "bar")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When multiple command arguments are passed
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--foo",
		"bar",
		"--ping",
		"pong",
		"test",
	}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	cmd = append(cmd, "--ping", "pong", "test")
	assert.Equal(t, cmd, deployment.Spec.Template.Spec.Containers[0].Command)

	// When one of the ExtraCommandArgs already exists in cmd with same or different value
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{
		"--argocd-repo-server",
		"foo.scv.cluster.local:6379",
	}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, baseCommand, deployment.Spec.Template.Spec.Containers[0].Command)

	// Remove all the command arguments that were added.
	a.Spec.ApplicationSet.ExtraCommandArgs = []string{}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, baseCommand, deployment.Spec.Template.Spec.Containers[0].Command)
}

func TestArgoCDApplicationSetEnv(t *testing.T) {
	a := makeTestArgoCD()
	a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	defaultEnv := []corev1.EnvVar{
		{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "",
					FieldPath:  "metadata.namespace",
				},
			},
		},
	}

	// Pass an environment variable using Argo CD CR.
	customEnv := []corev1.EnvVar{
		{
			Name:  "foo",
			Value: "bar",
		},
	}
	a.Spec.ApplicationSet.Env = customEnv

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.reconcileApplicationSetController(a))

	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	expectedEnv := append(defaultEnv, customEnv...)
	assert.Equal(t, expectedEnv, deployment.Spec.Template.Spec.Containers[0].Env)

	// Remove all the env vars that were added.
	a.Spec.ApplicationSet.Env = []corev1.EnvVar{}

	assert.NoError(t, r.reconcileApplicationSetController(a))
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-applicationset-controller",
			Namespace: a.Namespace,
		},
		deployment))

	assert.Equal(t, defaultEnv, deployment.Spec.Template.Spec.Containers[0].Env)
}
