package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	testclient "k8s.io/client-go/kubernetes/fake"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	_, err := r.reconcileNotificationsRole(a)
	assert.NoError(t, err)

	testRole := &rbacv1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testRole))

	desiredPolicyRules := policyRuleForNotificationsController()

	assert.Equal(t, desiredPolicyRules, testRole.Rules)

	a.Spec.Notifications.Enabled = false
	_, err = r.reconcileNotificationsRole(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	desiredSa, err := r.reconcileNotificationsServiceAccount(a)
	assert.NoError(t, err)

	testSa := &v1.ServiceAccount{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, testSa))

	assert.Equal(t, testSa.Name, desiredSa.Name)

	a.Spec.Notifications.Enabled = false
	_, err = r.reconcileNotificationsServiceAccount(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "role-name"}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name"}}

	err := r.reconcileNotificationsRoleBinding(a, role, sa)
	assert.NoError(t, err)

	roleBinding := &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(
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

	err = r.Get(context.TODO(), types.NamespacedName{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	sa := v1.ServiceAccount{}

	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	// Ensure the created Deployment has the expected properties
	assert.Equal(t, deployment.Spec.Template.Spec.ServiceAccountName, sa.Name)

	want := []v1.Container{{
		Command:         []string{"argocd-notifications", "--loglevel", "info", "--logformat", "text", "--argocd-repo-server", "argocd-repo-server.argocd.svc.cluster.local:8081"},
		Image:           argoutil.CombineImageTag(common.ArgoCDDefaultArgoImage, common.ArgoCDDefaultArgoVersion),
		ImagePullPolicy: v1.PullIfNotPresent,
		Name:            "argocd-notifications-controller",
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts: []v1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "argocd-repo-server-tls",
				MountPath: "/app/config/reposerver/tls",
			},
		},
		Resources:  v1.ResourceRequirements{},
		WorkingDir: "/app",
		LivenessProbe: &v1.Probe{
			ProbeHandler: v1.ProbeHandler{
				TCPSocket: &v1.TCPSocketAction{
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
			Name: "argocd-repo-server-tls",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
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

	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      generateResourceName(common.ArgoCDNotificationsControllerComponent, a),
		Namespace: a.Namespace,
	}, deployment)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_CreateMetricsService(t *testing.T) {
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := monitoringv1.AddToScheme(r.Scheme)
	assert.NoError(t, err)

	err = r.reconcileNotificationsMetricsService(a)
	assert.NoError(t, err)

	testService := &v1.Service{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, "notifications-controller-metrics"),
		Namespace: a.Namespace,
	}, testService))

	assert.Equal(t, testService.Labels["app.kubernetes.io/name"],
		fmt.Sprintf("%s-%s", a.Name, "notifications-controller-metrics"))

	assert.Equal(t, testService.Spec.Selector["app.kubernetes.io/name"],
		fmt.Sprintf("%s-%s", a.Name, "notifications-controller"))

	assert.Equal(t, testService.Spec.Ports[0].Port, int32(9001))
	assert.Equal(t, testService.Spec.Ports[0].TargetPort, intstr.IntOrString{
		IntVal: int32(9001),
	})
	assert.Equal(t, testService.Spec.Ports[0].Protocol, v1.Protocol("TCP"))
	assert.Equal(t, testService.Spec.Ports[0].Name, "metrics")
}

func TestReconcileNotifications_CreateServiceMonitor(t *testing.T) {

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	err := monitoringv1.AddToScheme(sch)
	assert.NoError(t, err)
	err = v1alpha1.AddToScheme(sch)
	assert.NoError(t, err)

	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Notifications controller service monitor should not be created when Prometheus API is not found.
	prometheusAPIFound = false
	err = r.reconcileNotificationsController(a)
	assert.NoError(t, err)

	testServiceMonitor := &monitoringv1.ServiceMonitor{}
	assert.Error(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, "notifications-controller-metrics"),
		Namespace: a.Namespace,
	}, testServiceMonitor))

	// Prometheus API found, Verify notification controller service monitor exists.
	prometheusAPIFound = true
	err = r.reconcileNotificationsController(a)
	assert.NoError(t, err)

	testServiceMonitor = &monitoringv1.ServiceMonitor{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      fmt.Sprintf("%s-%s", a.Name, "notifications-controller-metrics"),
		Namespace: a.Namespace,
	}, testServiceMonitor))

	assert.Equal(t, testServiceMonitor.Labels["release"], "prometheus-operator")

	assert.Equal(t, testServiceMonitor.Spec.Endpoints[0].Port, "metrics")
	assert.Equal(t, testServiceMonitor.Spec.Endpoints[0].Scheme, "http")
	assert.Equal(t, testServiceMonitor.Spec.Endpoints[0].Interval, monitoringv1.Duration("30s"))
	assert.Equal(t, testServiceMonitor.Spec.Selector.MatchLabels["app.kubernetes.io/name"],
		fmt.Sprintf("%s-%s", a.Name, "notifications-controller-metrics"))
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.reconcileNotificationsSecret(a)
	assert.NoError(t, err)

	testSecret := &v1.Secret{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name:      "argocd-notifications-secret",
		Namespace: a.Namespace,
	}, testSecret))

	a.Spec.Notifications.Enabled = false
	err = r.reconcileNotificationsSecret(a)
	assert.NoError(t, err)
	secret := &v1.Secret{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "argocd-notifications-secret", Namespace: a.Namespace}, secret)
	assertNotFound(t, err)
}

func TestReconcileNotifications_testEnvVars(t *testing.T) {

	envMap := []v1.EnvVar{
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Get(
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
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
		"--logformat",
		"text",
		"--argocd-repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
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
	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile back
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Get(
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

func TestReconcileNotifications_testLogFormat(t *testing.T) {
	testLogFormat := "json"
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
		a.Spec.Notifications.LogFormat = testLogFormat
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	sa := v1.ServiceAccount{}
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	deployment := &appsv1.Deployment{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	expectedCMD := []string{
		"argocd-notifications",
		"--loglevel",
		"info",
		"--logformat",
		"json",
		"--argocd-repo-server",
		"argocd-repo-server.argocd.svc.cluster.local:8081",
	}

	if diff := cmp.Diff(expectedCMD, deployment.Spec.Template.Spec.Containers[0].Command); diff != "" {
		t.Fatalf("failed to reconcile notifications-controller deployment logFormat:\n%s", diff)
	}

	// Verify any manual updates to the logFormat should be overridden by the operator.
	unwantedCommand := []string{
		"argocd-notifications",
		"--logformat",
		"text",
	}

	deployment.Spec.Template.Spec.Containers[0].Command = unwantedCommand
	assert.NoError(t, r.Update(context.TODO(), deployment))

	// Reconcile back
	assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

	// Get the updated deployment
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      a.Name + "-notifications-controller",
			Namespace: a.Namespace,
		},
		deployment))

	if diff := cmp.Diff(expectedCMD, deployment.Spec.Template.Spec.Containers[0].Command); diff != "" {
		t.Fatalf("operator failed to override the manual changes to notification controller logFormat:\n%s", diff)
	}
}
