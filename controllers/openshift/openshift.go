package openshift

import (
	"context"
	"os"
	"strings"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("openshift_controller_argocd")

func init() {
	argocd.Register(reconcilerHook)
}

func reconcilerHook(cr *argoprojv1alpha1.ArgoCD, v interface{}, hint string) error {

	logv := log.WithValues("ArgoCD Namespace", cr.Namespace, "ArgoCD Name", cr.Name)
	switch o := v.(type) {
	case *rbacv1.ClusterRole:
		if o.ObjectMeta.Name == argocd.GenerateUniqueResourceName("argocd-application-controller", cr) {
			logv.Info("configuring openshift cluster config policy rules")
			o.Rules = policyRulesForClusterConfig()
		}
	case *appsv1.Deployment:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-redis" {
			logv.Info("configuring openshift redis")
			o.Spec.Template.Spec.Containers[0].Args = append(getArgsForRedhatRedis(), o.Spec.Template.Spec.Containers[0].Args...)
		} else if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-redis-ha-haproxy" {
			logv.Info("configuring openshift redis haproxy")
			o.Spec.Template.Spec.Containers[0].Command = append(getCommandForRedhatRedisHaProxy(), o.Spec.Template.Spec.Containers[0].Command...)
		}
	case *[]rbacv1.PolicyRule:
		if hint == "policyRuleForRedisHa" {
			logv.Info("configuring policy rule for Redis HA")
			*o = append(*o, getPolicyRuleForRedisHa())
		}
	case *appsv1.StatefulSet:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-redis-ha-server" {
			logv.Info("configuring openshift redis-ha-server stateful set")
			for index, _ := range o.Spec.Template.Spec.Containers {
				if o.Spec.Template.Spec.Containers[index].Name == "redis" {
					o.Spec.Template.Spec.Containers[index].Args = getArgsForRedhatHaRedisServer()
					o.Spec.Template.Spec.Containers[index].Command = []string{}
				} else if o.Spec.Template.Spec.Containers[index].Name == "sentinel" {
					o.Spec.Template.Spec.Containers[index].Args = getArgsForRedhatHaRedisSentinel()
					o.Spec.Template.Spec.Containers[index].Command = []string{}
				}
			}
			o.Spec.Template.Spec.InitContainers[0].Args = getArgsForRedhatHaRedisInitContainer()
			o.Spec.Template.Spec.InitContainers[0].Command = []string{}
		}
	case *corev1.Secret:
		if allowedNamespace(cr.ObjectMeta.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
			logv.Info("configuring cluster secret with empty namespaces to allow cluster resources")
			delete(o.Data, "namespaces")
		}
	case *rbacv1.Role:
		if o.ObjectMeta.Name == cr.Name+"-"+"argocd-application-controller" {
			logv.Info("configuring policy rule for Application Controller")

			// can move this to somewhere common eventually, maybe init()
			k8sClient, err := initK8sClient()
			if err != nil {
				logv.Error(err, "failed to initialize kube client")
				return err
			}

			clusterRole, err := k8sClient.RbacV1().ClusterRoles().Get(context.TODO(), "admin", metav1.GetOptions{})
			if err != nil {
				logv.Error(err, "failed to retrieve Cluster Role admin")
				return err
			}
			policyRules := getPolicyRuleForApplicationController()
			policyRules = append(policyRules, clusterRole.Rules...)
			o.Rules = policyRules
		}
	}
	return nil
}

func getPolicyRuleForRedisHa() rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups: []string{
			"security.openshift.io",
		},
		ResourceNames: []string{
			"nonroot",
		},
		Resources: []string{
			"securitycontextconstraints",
		},
		Verbs: []string{
			"use",
		},
	}
}

func getPolicyRuleForApplicationController() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}

// For OpenShift, we use a custom build of Redis provided by Red Hat
// which requires additional args in comparison to stock redis.
func getArgsForRedhatRedis() []string {
	return []string{
		"redis-server",
		"--protected-mode",
		"no",
	}
}

// For OpenShift, we use a custom build of haproxy provided by Red Hat
// which requires a command as opposed to args in stock haproxy.
func getCommandForRedhatRedisHaProxy() []string {
	return []string{
		"haproxy",
		"-f",
		"/usr/local/etc/haproxy/haproxy.cfg",
	}
}

// For OpenShift, we use a custom build of Redis provided by Red Hat
// which requires additional args in comparison to stock redis.
func getArgsForRedhatHaRedisServer() []string {
	return []string{
		"redis-server",
		"/data/conf/redis.conf",
	}
}

// For OpenShift, we use a custom build of Redis provided by Red Hat
// which requires additional args in comparison to stock redis.
func getArgsForRedhatHaRedisSentinel() []string {
	return []string{
		"redis-sentinel",
		"/data/conf/sentinel.conf",
	}
}

// For OpenShift, we use a custom build of Redis provided by Red Hat
// which requires additional args in comparison to stock redis.
func getArgsForRedhatHaRedisInitContainer() []string {
	return []string{
		"sh",
		"/readonly-config/init.sh",
	}
}

// policyRulesForClusterConfig defines rules for cluster config.
func policyRulesForClusterConfig() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			NonResourceURLs: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"list",
			},
		},
		{
			APIGroups: []string{
				"operators.coreos.com",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"operator.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"user.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"config.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"console.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"namespaces",
				"persistentvolumeclaims",
				"persistentvolumes",
				"configmaps",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"rbac.authorization.k8s.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"storage.k8s.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"machine.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"machineconfig.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"compliance.openshift.io",
			},
			Resources: []string{
				"scansettingbindings",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func allowedNamespace(current string, namespaces string) bool {

	clusterConfigNamespaces := splitList(namespaces)
	if len(clusterConfigNamespaces) > 0 {
		if clusterConfigNamespaces[0] == "*" {
			return true
		}

		for _, n := range clusterConfigNamespaces {
			if n == current {
				return true
			}
		}
	}
	return false
}

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

func initK8sClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	kClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return kClient, nil
}
