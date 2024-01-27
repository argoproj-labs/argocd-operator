package test

import (
	"errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TestArgoCDName        = "test-argocd"
	TestName              = "test-name"
	TestInstance          = "test-instance"
	TestInstanceNamespace = "test-instance-ns"
	TestNamespace         = "test-ns"
	TestComponent         = "test-component"
	TestApplicationName   = "test-application-name"
	TestKey               = "test-key"
	TestVal               = "test-val"
	TestValMutated        = "test-val-mutated"
	TestNameMutated       = "test-name-mutated"
)

var (
	TestKVP = map[string]string{
		TestKey: TestVal,
	}
	TestKVPMutated = map[string]string{
		TestKey: TestValMutated,
	}
)

func TestMutationFuncFailed(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	return errors.New("test-mutation-error")
}

func TestMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	return nil
}

type argoCDOpt func(*argoproj.ArgoCD)

func MakeTestArgoCD(opts ...argoCDOpt) *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestArgoCDName,
			Namespace: TestNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

type namespaceOpt func(*corev1.Namespace)

func MakeTestNamespace(opts ...namespaceOpt) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   TestNamespace,
			Labels: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(ns)
	}
	return ns
}

type statefulSetOpt func(*appsv1.StatefulSet)

func MakeTestStatefulSet(opts ...statefulSetOpt) *appsv1.StatefulSet {
	desiredStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{},
		},
	}

	for _, opt := range opts {
		opt(desiredStatefulSet)
	}
	return desiredStatefulSet
}

type deploymentOpt func(*appsv1.Deployment)

func MakeTestDeployment(opts ...deploymentOpt) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{},
		},
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

type hpaOpt func(*autoscalingv1.HorizontalPodAutoscaler)

func MakeTestHPA(opts ...hpaOpt) *autoscalingv1.HorizontalPodAutoscaler {
	hpa := &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: autoscalingv1.HorizontalPodAutoscalerSpec{},
	}
	for _, o := range opts {
		o(hpa)
	}
	return hpa
}

type podOpt func(*corev1.Pod)

func MakeTestPod(opts ...podOpt) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestName,
			Namespace: TestNamespace,
		},
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

type serviceOpt func(*corev1.Service)

func MakeTestService(opts ...serviceOpt) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type configMapOpt func(*corev1.ConfigMap)

func MakeTestConfigMap(opts ...configMapOpt) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Data: make(map[string]string),
	}
	for _, o := range opts {
		o(cm)
	}
	return cm
}

type secretOpt func(*corev1.Secret)

func MakeTestSecret(opts ...secretOpt) *corev1.Secret {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		StringData: map[string]string{},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type roleOpt func(*rbacv1.Role)

func MakeTestRole(opts ...roleOpt) *rbacv1.Role {
	r := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Rules: TestRules,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func MakeTestRoleRef(name string) rbacv1.RoleRef {
	return rbacv1.RoleRef{
		Kind:     "Role",
		Name:     name,
		APIGroup: "rbac.authorization.k8s.io",
	}
}

func MakeTestSubjects(subs ...types.NamespacedName) []rbacv1.Subject {
	subjects := []rbacv1.Subject{}

	for _, subj := range subs {
		subjects = append(subjects, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      subj.Name,
			Namespace: subj.Namespace,
		})
	}
	return subjects
}

var (
	TestRules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"create",
			},
		},
	}
)

type roleBindingOpt func(*rbacv1.RoleBinding)

func MakeTestRoleBinding(opts ...roleBindingOpt) *rbacv1.RoleBinding {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(rb)
	}
	return rb
}

type clusterRoleOpt func(*rbacv1.ClusterRole)

func MakeTestClusterRole(opts ...clusterRoleOpt) *rbacv1.ClusterRole {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Rules: TestRules,
	}
	for _, o := range opts {
		o(cr)
	}
	return cr
}

type clusterRoleBindingOpt func(*rbacv1.ClusterRoleBinding)

func MakeTestClusterRoleBinding(opts ...clusterRoleBindingOpt) *rbacv1.ClusterRoleBinding {
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     TestName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	for _, o := range opts {
		o(crb)
	}
	return crb
}

type serviceAccountOpt func(*corev1.ServiceAccount)

func MakeTestServiceAccount(opts ...serviceAccountOpt) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(sa)
	}
	return sa
}

type serviceMonitorOpt func(*monitoringv1.ServiceMonitor)

func MakeTestServiceMonitor(opts ...serviceMonitorOpt) *monitoringv1.ServiceMonitor {
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: TestKVP,
			},
		},
	}
	for _, o := range opts {
		o(sm)
	}
	return sm
}

type prometheusRuleOpt func(*monitoringv1.PrometheusRule)

func MakeTestPrometheusRule(opts ...prometheusRuleOpt) *monitoringv1.PrometheusRule {
	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        TestName,
			Namespace:   TestNamespace,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(pr)
	}
	return pr
}
