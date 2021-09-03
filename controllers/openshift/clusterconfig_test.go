package openshift

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

const (
	testNamespace             = "argocd"
	testArgoCDName            = "argocd"
	testApplicationController = "argocd-application-controller"
	testDummyNameSpace        = "dummy"
)

func makeTestPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"foo.example.com",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func makeTestArgoCDForClusterConfig() *argoprojv1alpha1.ArgoCD {
	a := makeTestArgoCD()
	a.Namespace = testNamespace
	return a
}

func makeTestArgoCD() *argoprojv1alpha1.ArgoCD {
	a := &argoprojv1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testDummyNameSpace,
		},
	}
	return a
}

func setClusterConfigNamespaces() {
	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", "argocd,foo,bar")
}

func unSetClusterConfigNamespaces() {
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
}

func makeTestClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: testApplicationController,
		},
		Rules: makeTestPolicyRules(),
	}
}

func makeTestDeployment() *appsv1.Deployment {
	var replicas int32 = 1
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testApplicationController,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "name",
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"testing"},
							Image:   "test-image",
						},
					},
				},
			},
		},
	}
}

func makeTestRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
		Subjects: []rbacv1.Subject{},
		RoleRef:  rbacv1.RoleRef{},
	}
}

func newStatefulSetWithSuffix(suffix string, component string, cr *argoprojv1alpha1.ArgoCD) *appsv1.StatefulSet {
	return newStatefulSetWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}

func newStatefulSetWithName(name string, component string, cr *argoprojv1alpha1.ArgoCD) *appsv1.StatefulSet {
	ss := newStatefulSet(cr)
	ss.ObjectMeta.Name = name

	lbls := ss.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	ss.ObjectMeta.Labels = lbls

	return ss
}

func newStatefulSet(cr *argoprojv1alpha1.ArgoCD) *appsv1.StatefulSet {
	ss := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}

	ss.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Args: []string{
				"/data/conf/redis.conf",
			},
			Command: []string{
				"redis-server",
			},
			Image:           "dummy-image",
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "redis",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			}},
			Resources: corev1.ResourceRequirements{},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
		{
			Args: []string{
				"/data/conf/sentinel.conf",
			},
			Command: []string{
				"redis-sentinel",
			},
			Image:           "dummy-image",
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(common.ArgoCDDefaultRedisSentinelPort),
					},
				},
				InitialDelaySeconds: int32(15),
			},
			Name: "sentinel",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisSentinelPort,
				Name:          "sentinel",
			}},
			Resources: corev1.ResourceRequirements{},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
			},
		},
	}

	ss.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly-config/init.sh",
		},
		Command: []string{
			"sh",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SENTINEL_ID_0",
				Value: "25b71bd9d0e4a51945d8422cab53f27027397c12",
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "896627000a81c7bdad8dbdcffd39728c9c17b309",
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "3acbca861108bc47379b71b1d87d1c137dce591f",
			},
		},
		Image:           "dummy-image",
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Resources:       corev1.ResourceRequirements{},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/readonly-config",
				Name:      "config",
				ReadOnly:  true,
			},
			{
				MountPath: "/data",
				Name:      "data",
			},
		},
	}}

	return &ss
}
