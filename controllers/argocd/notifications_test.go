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

func TestReconcileNotifications_CreateClusterRole(t *testing.T) {
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

	_, err := r.reconcileNotificationsClusterRole(a)
	assert.NoError(t, err)

	testClusterRole := &rbacv1.ClusterRole{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDNotificationsControllerComponent, a),
	}, testClusterRole))

	desiredPolicyRules := policyRuleForNotificationsControllerClusterRole()

	assert.Equal(t, desiredPolicyRules, testClusterRole.Rules)

	a.Spec.Notifications.Enabled = false
	_, err = r.reconcileNotificationsClusterRole(a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDNotificationsControllerComponent, a),
	}, testClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_CreateClusterRoleBinding(t *testing.T) {
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

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "cluster-role-name"}}
	sa := &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "sa-name", Namespace: a.Namespace}}

	err := r.reconcileNotificationsClusterRoleBinding(a, clusterRole, sa)
	assert.NoError(t, err)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	assert.NoError(t, r.Get(
		context.TODO(),
		types.NamespacedName{
			Name: GenerateUniqueResourceName(common.ArgoCDNotificationsControllerComponent, a),
		},
		clusterRoleBinding))

	assert.Equal(t, clusterRoleBinding.RoleRef.Name, clusterRole.Name)
	assert.Equal(t, clusterRoleBinding.RoleRef.Kind, "ClusterRole")
	assert.Equal(t, clusterRoleBinding.Subjects[0].Name, sa.Name)
	assert.Equal(t, clusterRoleBinding.Subjects[0].Namespace, sa.Namespace)

	a.Spec.Notifications.Enabled = false
	err = r.reconcileNotificationsClusterRoleBinding(a, clusterRole, sa)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{
		Name: GenerateUniqueResourceName(common.ArgoCDNotificationsControllerComponent, a),
	}, clusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_Deployments_Command(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name           string
		argocdSpec     argoproj.ArgoCDSpec
		expectedCmd    []string
		notExpectedCmd []string
	}{
		{
			name: "Notifications contained in spec.sourceNamespaces",

			argocdSpec: argoproj.ArgoCDSpec{
				Notifications: argoproj.ArgoCDNotifications{
					Enabled:          true,
					SourceNamespaces: []string{"foo", "bar"},
				},
				SourceNamespaces: []string{"foo", "bar"},
			},
			expectedCmd: []string{"--application-namespaces", "foo,bar", "--self-service-notification-enabled", "true"},
		},
		{
			name: "Only notifications contained in spec.sourceNamespaces",
			argocdSpec: argoproj.ArgoCDSpec{
				Notifications: argoproj.ArgoCDNotifications{
					Enabled:          true,
					SourceNamespaces: []string{"foo"},
				},
				SourceNamespaces: []string{"foo", "bar"},
			},
			expectedCmd: []string{"--application-namespaces", "foo", "--self-service-notification-enabled", "true"},
		},
		{
			name: "Empty spec.sourceNamespaces, no application namespaces arg",
			argocdSpec: argoproj.ArgoCDSpec{
				Notifications: argoproj.ArgoCDNotifications{
					Enabled:          true,
					SourceNamespaces: []string{"foo"},
				},
				SourceNamespaces: []string{},
			},
			expectedCmd:    []string{},
			notExpectedCmd: []string{"--application-namespaces", "foo", "--self-service-notification-enabled", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD()
			ns1 := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
			}
			ns2 := v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar",
				},
			}
			resObjs := []client.Object{a, &ns1, &ns2}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			err := r.Create(context.Background(), cm, &client.CreateOptions{})
			assert.NoError(t, err)

			a.Spec = test.argocdSpec

			sa := v1.ServiceAccount{}
			assert.NoError(t, r.reconcileNotificationsDeployment(a, &sa))

			deployment := &appsv1.Deployment{}
			assert.NoError(t, r.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      "argocd-notifications-controller",
					Namespace: a.Namespace,
				},
				deployment))

			cmds := deployment.Spec.Template.Spec.Containers[0].Command
			for _, c := range test.expectedCmd {
				assert.True(t, contains(cmds, c))
			}
			for _, c := range test.notExpectedCmd {
				assert.False(t, contains(cmds, c))
			}
		})
	}
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

func TestArgoCDNotifications_getNotificationsSourceNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name               string
		notificationsField argoproj.ArgoCDNotifications
		expected           []string
	}{
		{
			name:               "No Notifications configured",
			notificationsField: argoproj.ArgoCDNotifications{},
			expected:           []string(nil),
		},
		{
			name: "Notifications enabled No notifications source namespaces",
			notificationsField: argoproj.ArgoCDNotifications{
				Enabled: true,
			},
			expected: []string(nil),
		},
		{
			name: "Notifications enabled and notifications source namespaces",
			notificationsField: argoproj.ArgoCDNotifications{
				Enabled:          true,
				SourceNamespaces: []string{"foo", "bar"},
			},
			expected: []string{"foo", "bar"},
		},
		{
			name: "Notifications disabled and notifications source namespaces",
			notificationsField: argoproj.ArgoCDNotifications{
				Enabled:          false,
				SourceNamespaces: []string{"foo", "bar"},
			},
			expected: []string(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			a := makeTestArgoCD()
			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			cm := newConfigMapWithName(getCAConfigMapName(a), a)
			err := r.Create(context.Background(), cm, &client.CreateOptions{})
			assert.NoError(t, err)

			a.Spec.Notifications = test.notificationsField

			actual := r.getNotificationsSourceNamespaces(a)
			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestArgoCDNotifications_setManagedNotificationsSourceNamespaces(t *testing.T) {
	a := makeTestArgoCD()
	ns1 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-1",
			Labels: map[string]string{
				common.ArgoCDNotificationsManagedByClusterArgoCDLabel: testNamespace,
			},
		},
	}
	ns2 := v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace-2",
		},
	}

	resObjs := []client.Object{a, &ns1, &ns2}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err := r.setManagedNotificationsSourceNamespaces(a)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(r.ManagedNotificationsSourceNamespaces))
	assert.Contains(t, r.ManagedNotificationsSourceNamespaces, "test-namespace-1")
}

func TestNotifications_removeUnmanagedNotificationsSourceNamespaceResources(t *testing.T) {
	ns1 := "foo"
	ns2 := "bar"
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{ns1, ns2},
		Notifications: argoproj.ArgoCDNotifications{
			Enabled:          true,
			SourceNamespaces: []string{ns1, ns2},
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	err := v1alpha1.AddToScheme(sch)
	assert.NoError(t, err)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	err = createNamespace(r, ns1, "")
	assert.NoError(t, err)
	err = createNamespace(r, ns2, "")
	assert.NoError(t, err)

	// create resources
	err = r.reconcileNotificationsSourceNamespacesResources(a)
	assert.NoError(t, err)

	// remove notifications ns
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{ns2},
		Notifications: argoproj.ArgoCDNotifications{
			Enabled:          true,
			SourceNamespaces: []string{ns1, ns2},
		},
	}

	// clean up unmanaged namespaces resources
	err = r.removeUnmanagedNotificationsSourceNamespaceResources(a)
	assert.NoError(t, err)

	// resources shouldn't exist in ns1
	resName := getResourceNameForNotificationsSourceNamespaces(a)

	role := &rbacv1.Role{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: resName, Namespace: ns1}, role)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	roleBinding := &rbacv1.RoleBinding{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: resName, Namespace: ns1}, roleBinding)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	// NotificationsConfiguration CR should be deleted from ns1
	notifCfg := &v1alpha1.NotificationsConfiguration{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: DefaultNotificationsConfigurationInstanceName, Namespace: ns1}, notifCfg)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	// notifications tracking label should be removed
	namespace := &v1.Namespace{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: ns1}, namespace)
	assert.NoError(t, err)
	_, found := namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel]
	assert.False(t, found)

	// resources in ns2 shouldn't be touched

	role = &rbacv1.Role{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: resName, Namespace: ns2}, role)
	assert.NoError(t, err)

	roleBinding = &rbacv1.RoleBinding{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: resName, Namespace: ns2}, roleBinding)
	assert.NoError(t, err)

	// NotificationsConfiguration CR should still exist in ns2
	notifCfg = &v1alpha1.NotificationsConfiguration{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: DefaultNotificationsConfigurationInstanceName, Namespace: ns2}, notifCfg)
	assert.NoError(t, err)

	namespace = &v1.Namespace{}
	err = r.Get(context.TODO(), client.ObjectKey{Name: ns2}, namespace)
	assert.NoError(t, err)
	val, found := namespace.Labels[common.ArgoCDNotificationsManagedByClusterArgoCDLabel]
	assert.True(t, found)
	assert.Equal(t, a.Namespace, val)
}

func TestReconcileNotifications_NotificationsConfigurationInSourceNamespaceWhenDisabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	sourceNamespace := "ns1"
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = false
		a.Spec.Notifications.SourceNamespaces = []string{sourceNamespace}
		a.Spec.SourceNamespaces = []string{sourceNamespace}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	err := v1alpha1.AddToScheme(sch)
	assert.NoError(t, err)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Create the source namespace
	err = createNamespace(r, sourceNamespace, "")
	assert.NoError(t, err)

	// Reconcile should not create NotificationsConfiguration CR when notifications are disabled
	err = r.reconcileSourceNamespaceNotificationsConfigurationCR(a, sourceNamespace)
	assert.NoError(t, err)

	// Verify NotificationsConfiguration CR does not exist
	notifCfg := &v1alpha1.NotificationsConfiguration{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      DefaultNotificationsConfigurationInstanceName,
		Namespace: sourceNamespace,
	}, notifCfg)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileNotifications_SourceNamespaceResourcesIncludeNotificationsConfiguration(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	sourceNamespace := "ns1"
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Notifications.Enabled = true
		a.Spec.Notifications.SourceNamespaces = []string{sourceNamespace}
		a.Spec.SourceNamespaces = []string{sourceNamespace}
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	err := v1alpha1.AddToScheme(sch)
	assert.NoError(t, err)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	// Create the source namespace
	err = createNamespace(r, sourceNamespace, "")
	assert.NoError(t, err)

	// Reconcile source namespace resources (this should create NotificationsConfiguration CR)
	err = r.reconcileNotificationsSourceNamespacesResources(a)
	assert.NoError(t, err)

	// Verify NotificationsConfiguration CR was created in source namespace
	notifCfg := &v1alpha1.NotificationsConfiguration{}
	err = r.Get(context.TODO(), types.NamespacedName{
		Name:      DefaultNotificationsConfigurationInstanceName,
		Namespace: sourceNamespace,
	}, notifCfg)
	assert.NoError(t, err)
	assert.Equal(t, DefaultNotificationsConfigurationInstanceName, notifCfg.Name)
	assert.Equal(t, sourceNamespace, notifCfg.Namespace)
}
