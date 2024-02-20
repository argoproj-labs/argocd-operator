package test

import (
	"errors"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	TestCert              = `-----BEGIN CERTIFICATE-----
	MIIDyDCCArCgAwIBAgIJAKX+Np0w4tdQMA0GCSqGSIb3DQEBCwUAMIGbMQswCQYD
	VQQGEwJVUzELMAkGA1UECAwCTkwxFjAUBgNVBAcMDU5ldyBZb3JrIENpdHkxEjAQ
	BgNVBAoMCU15IENvbXBhbnkxEDAOBgNVBAsMB09jdG9iZXIxHjAcBgNVBAMMFU15
	IENvbXBhbnkgU2VydmljZSBDQTAeFw0yMjAxMDYxODM3MzZaFw0yMjAyMTAxODM3
	MzZaMIGbMQswCQYDVQQGEwJVUzELMAkGA1UECAwCTkwxFjAUBgNVBAcMDU5ldyBZ
	b3JrIENpdHkxEjAQBgNVBAoMCU15IENvbXBhbnkxEDAOBgNVBAsMB09jdG9iZXIx
	HjAcBgNVBAMMFU15IENvbXBhbnkgU2VydmljZSBDQTCCASIwDQYJKoZIhvcNAQEB
	BQADggEPADCCAQoCggEBAKlKY9O0W5WQyGs5NXpgu98tMmQ20NKbFCXKuSwR5WkP
	zA63o9iQzOS4t1Kf7KlyvB4LC4Ow8/qo7e4dnuXuT/ZXqWbYY46SorGX0RmM5U+
	L86A6Oo3n0IiMmkDZ9svwxYNpzsqBNBCl0jDSqyUcPhZtV2o2Sb9WlqG6MbfWMPu
	G/q6wLTTt8apGXid1DtfPLDl6mPDezWx2FncMyH7Wdz/gvA8JDKRIREHj+sDFWTJ
	3a3LMWkgihFiGnmbMW0sg6VQvmsBp4JzdRFTu7Z+olpDYcR6GgCGaW15A96csaX1
	NlG0vtCpU/MDUhtDuj+cIAVoV1uEzkkNZbNQs0p/eIsCAwEAAaNTMFEwHQYDVR0O
	BBYEFN/nLRCDjJ8FBBghXUg9CdtmU4uMIGZBgNVHSMEgZEwgY6AFN/nLRCDjJ8FB
	BghXUg9CdtmU4uoXykRjBEMQswCQYDVQQGEwJVUzELMAkGA1UECAwCTkwxFjAUB
	gNVBAcMDU5ldyBZb3JrIENpdHkxEjAQBgNVBAoMCU15IENvbXBhbnkxEDAOBgNV
	BAsMB09jdG9iZXIxHjAcBgNVBAMMFU15IENvbXBhbnkgU2VydmljZSBDQQIJAKX+
	Np0w4tdQMAwGA1UdEwQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAK8c+6/baaM
	Pj2l92Vxl2lCwK4nZVztHn19kRUzIb8VsFtHnNtnFIdEXfZb+KFq1SV8pSp2tVeo
	jHyU98uz5ANwGofhVL+PVrI+4zz7lvbSIS9gRggTS92aWK0DxqGt4TT8K9OmffwI
	JL/r6pCSnE46AtkCwh8EKIuYq1+aF+DGE4ZZrWuFyCLb0ro8WMH75tTHUab8QGCA
	zDbZJix2s0R8Bm1YbSEePh9BCLDl6QUQbLU9ZtHrgK4m8nhrjUd/hnmDH/1RbYap
	w/yXv7R0p7T5ib6pCzA/1F7qTatpZbyJJTME+n4xUsiZfEC/VXNX3ApKlIhtGhTu
	czrfR8E2oUo=
-----END CERTIFICATE-----`
)

var (
	TestKVP = map[string]string{
		TestKey: TestVal,
	}
	TestKVPMutated = map[string]string{
		TestKey: TestValMutated,
	}
)

func MakeTestReconcilerClient(sch *runtime.Scheme, resObjs, subresObjs []client.Object, runtimeObj []runtime.Object) client.Client {
	client := fake.NewClientBuilder().WithScheme(sch)
	if len(resObjs) > 0 {
		client = client.WithObjects(resObjs...)
	}
	if len(subresObjs) > 0 {
		client = client.WithStatusSubresource(subresObjs...)
	}
	if len(runtimeObj) > 0 {
		client = client.WithRuntimeObjects(runtimeObj...)
	}
	return client.Build()
}

type SchemeOpt func(*runtime.Scheme)

func MakeTestReconcilerScheme(sOpts ...SchemeOpt) *runtime.Scheme {
	s := scheme.Scheme
	for _, opt := range sOpts {
		opt(s)
	}

	return s
}

func TestMutationFuncFailed(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	return errors.New("test-mutation-error")
}

func TestMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	return nil
}

type namespaceOpt func(*corev1.Namespace)

func MakeTestNamespace(ns *corev1.Namespace, opts ...namespaceOpt) *corev1.Namespace {
	if ns == nil {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   TestNamespace,
				Labels: make(map[string]string),
			},
		}
	}
	for _, o := range opts {
		o(ns)
	}
	return ns
}

type statefulSetOpt func(*appsv1.StatefulSet)

func MakeTestStatefulSet(ss *appsv1.StatefulSet, opts ...statefulSetOpt) *appsv1.StatefulSet {
	if ss == nil {
		ss = &appsv1.StatefulSet{
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
	}
	for _, opt := range opts {
		opt(ss)
	}
	return ss
}

type deploymentOpt func(*appsv1.Deployment)

func MakeTestDeployment(d *appsv1.Deployment, opts ...deploymentOpt) *appsv1.Deployment {
	if d == nil {
		d = &appsv1.Deployment{
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
	}
	for _, o := range opts {
		o(d)
	}
	return d
}

type hpaOpt func(*autoscalingv1.HorizontalPodAutoscaler)

func MakeTestHPA(hpa *autoscalingv1.HorizontalPodAutoscaler, opts ...hpaOpt) *autoscalingv1.HorizontalPodAutoscaler {
	if hpa == nil {
		hpa = &autoscalingv1.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Spec: autoscalingv1.HorizontalPodAutoscalerSpec{},
		}
	}
	for _, o := range opts {
		o(hpa)
	}
	return hpa
}

type podOpt func(*corev1.Pod)

func MakeTestPod(p *corev1.Pod, opts ...podOpt) *corev1.Pod {
	if p == nil {
		p = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TestName,
				Namespace: TestNamespace,
			},
		}
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

type serviceOpt func(*corev1.Service)

func MakeTestService(s *corev1.Service, opts ...serviceOpt) *corev1.Service {
	if s == nil {
		s = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
		}
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type configMapOpt func(*corev1.ConfigMap)

func MakeTestConfigMap(cm *corev1.ConfigMap, opts ...configMapOpt) *corev1.ConfigMap {
	if cm == nil {
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Data: make(map[string]string),
		}
	}
	for _, o := range opts {
		o(cm)
	}
	return cm
}

type secretOpt func(*corev1.Secret)

func MakeTestSecret(s *corev1.Secret, opts ...secretOpt) *corev1.Secret {
	if s == nil {
		s = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			StringData: map[string]string{},
		}
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type roleOpt func(*rbacv1.Role)

func MakeTestRole(r *rbacv1.Role, opts ...roleOpt) *rbacv1.Role {
	if r == nil {
		r = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Rules: TestRules,
		}
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

func MakeTestRoleBinding(rb *rbacv1.RoleBinding, opts ...roleBindingOpt) *rbacv1.RoleBinding {
	if rb == nil {
		rb = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
		}
	}
	for _, o := range opts {
		o(rb)
	}
	return rb
}

type clusterRoleOpt func(*rbacv1.ClusterRole)

func MakeTestClusterRole(cr *rbacv1.ClusterRole, opts ...clusterRoleOpt) *rbacv1.ClusterRole {
	if cr == nil {
		cr = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
			Rules: TestRules,
		}
	}
	for _, o := range opts {
		o(cr)
	}
	return cr
}

type clusterRoleBindingOpt func(*rbacv1.ClusterRoleBinding)

func MakeTestClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, opts ...clusterRoleBindingOpt) *rbacv1.ClusterRoleBinding {
	if crb == nil {
		crb = &rbacv1.ClusterRoleBinding{
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

func MakeTestServiceMonitor(sm *monitoringv1.ServiceMonitor, opts ...serviceMonitorOpt) *monitoringv1.ServiceMonitor {
	if sm == nil {
		sm = &monitoringv1.ServiceMonitor{
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
	}
	for _, o := range opts {
		o(sm)
	}
	return sm
}

type prometheusRuleOpt func(*monitoringv1.PrometheusRule)

func MakeTestPrometheusRule(pr *monitoringv1.PrometheusRule, opts ...prometheusRuleOpt) *monitoringv1.PrometheusRule {
	if pr == nil {
		pr = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:        TestName,
				Namespace:   TestNamespace,
				Labels:      make(map[string]string),
				Annotations: make(map[string]string),
			},
		}
	}
	for _, o := range opts {
		o(pr)
	}
	return pr
}
