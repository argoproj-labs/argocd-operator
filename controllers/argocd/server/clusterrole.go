package server

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sr *ServerReconciler) reconcileClusterRole() error {

	crName := getClusterRoleName(sr.Instance.Name, sr.Instance.Namespace)
	crLables := common.DefaultResourceLabels(crName, sr.Instance.Name, ServerControllerComponent)

	// ArgoCD instance is not cluster scoped, cleanup any existing cluster role & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRole(crName)
	}

	sr.Logger.Info("reconciling clusterRole")

	crRequest := permissions.ClusterRoleRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crName,
			Labels:      crLables,
			Annotations: sr.Instance.Annotations,
		},
		Rules:     getPolicyRuleForClusterRule(),
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredCR, err := permissions.RequestClusterRole(crRequest)
	if err != nil {
		sr.Logger.Error(err, "reconcileClusterRole: failed to request cluster role", "name", desiredCR.Name)
		sr.Logger.V(1).Info("reconcileClusterRole: one or more mutations could not be applied")
		return err
	}

	// cluster role doesn't exist, create it
	existingCR, err := permissions.GetClusterRole(desiredCR.Name, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterRole: failed to retrieve role", "name", desiredCR.Name)
			return err
		}

		if err = permissions.CreateClusterRole(desiredCR, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterRole: failed to create role", "name", desiredCR.Name)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterRole: role created", "name", desiredCR.Name)
		return nil
	}

	// difference in existing & desired role, reset it
	if !reflect.DeepEqual(existingCR.Rules, desiredCR.Rules) {
		existingCR.Rules = desiredCR.Rules
		if err = permissions.UpdateClusterRole(existingCR, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterRole: failed to update cluster role", "name", existingCR.Name)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterRole: role updated", "name", existingCR.Name)
	}

	// hpa found, no changes detected
	return nil
}

// getPolicyRuleForClusterRule returns policy rules for server cluster role
func getPolicyRuleForClusterRule() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"list",
			},
		},
	}
}

// deleteClusterRole will delete cluster role with given name.
func (sr *ServerReconciler) deleteClusterRole(name string) error {
	if err := permissions.DeleteClusterRole(name, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "deleteClusterRole: failed to delete cluster role", "name", name)
		return err
	}
	sr.Logger.V(0).Info("deleteClusterRole: cluster role deleted", "name", name)
	return nil
}
