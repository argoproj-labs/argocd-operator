package argorollouts

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"
)

func nameWithSuffix(suffix string, crName string) string {
	return fmt.Sprintf("%s-%s", crName, suffix)
}

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(cr *argoprojv1a1.ArgoRollouts) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
	}
}

// newServiceAccountWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newServiceAccountWithName(name string, cr *argoprojv1a1.ArgoRollouts) *corev1.ServiceAccount {
	sa := newServiceAccount(cr)
	sa.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	return sa
}

// newRole returns a new Role instance.
func newRole(name string, rules []v1.PolicyRule, cr *argoprojv1a1.ArgoRollouts) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateResourceName(name, cr),
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
		Rules: rules,
	}
}

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoRollouts) string {
	return cr.Name + "-" + argoComponentName
}

func labelsForCluster(cr *argoprojv1a1.ArgoRollouts) map[string]string {
	labels := common.DefaultLabels(cr.Name)
	return labels
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(cr *argoprojv1a1.ArgoRollouts) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cr.Name,
			Labels:      labelsForCluster(cr),
			Annotations: annotationsForCluster(cr),
			Namespace:   cr.Namespace,
		},
	}
}

// newRoleBindingWithname creates a new RoleBinding.
func newRoleBindingWithname(name string, cr *argoprojv1a1.ArgoRollouts) *v1.RoleBinding {
	roleBinding := newRoleBinding(cr)
	roleBinding.ObjectMeta.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	labels := roleBinding.ObjectMeta.Labels
	labels[common.ArgoCDKeyName] = name
	roleBinding.ObjectMeta.Labels = labels

	return roleBinding
}

// annotationsForCluster returns the annotations for all cluster resources.
func annotationsForCluster(cr *argoprojv1a1.ArgoRollouts) map[string]string {
	annotations := common.DefaultAnnotations(cr.Name, cr.Namespace)
	for key, val := range cr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}

// newDeployment returns a new Deployment instance for the given ArgoCD.
func newDeployment(cr *argoprojv1a1.ArgoRollouts) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newDeploymentWithName returns a new Deployment instance for the given ArgoCD using the given name.
func newDeploymentWithName(name string, component string, cr *argoprojv1a1.ArgoRollouts) *appsv1.Deployment {
	deploy := newDeployment(cr)
	deploy.ObjectMeta.Name = name

	lbls := deploy.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	deploy.ObjectMeta.Labels = lbls

	deploy.Spec = appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				common.ArgoCDKeyName: name,
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					common.ArgoCDKeyName: name,
				},
			},
			Spec: corev1.PodSpec{
				NodeSelector: common.DefaultNodeSelector(),
			},
		},
	}

	if cr.Spec.NodePlacement != nil {
		deploy.Spec.Template.Spec.NodeSelector = argoutil.AppendStringMap(deploy.Spec.Template.Spec.NodeSelector, cr.Spec.NodePlacement.NodeSelector)
		deploy.Spec.Template.Spec.Tolerations = cr.Spec.NodePlacement.Tolerations
	}
	return deploy
}

// newDeploymentWithSuffix returns a new Deployment instance for the given ArgoCD using the given suffix.
func newDeploymentWithSuffix(suffix string, component string, cr *argoprojv1a1.ArgoRollouts) *appsv1.Deployment {
	return newDeploymentWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), component, cr)
}
