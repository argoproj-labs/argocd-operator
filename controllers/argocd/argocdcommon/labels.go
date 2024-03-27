package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func GetSvcMonitorLabel() map[string]string {
	return map[string]string{
		common.PrometheusReleaseKey: common.PrometheusOperator,
	}
}

func GetResourceManagementLabel() map[string]string {
	return map[string]string{
		common.ArgoCDArgoprojKeyRBACType: common.ArgoCDRBACTypeResourceMananagement,
	}
}

func GetAppManagementLabel() map[string]string {
	return map[string]string{
		common.ArgoCDArgoprojKeyRBACType: common.ArgoCDRBACTypeAppManagement,
	}
}

func GetAppsetManagementLabel() map[string]string {
	return map[string]string{
		common.ArgoCDArgoprojKeyRBACType: common.ArgoCDRBACTypeAppSetManagement,
	}
}

func GetInstanceLabelRequirement(instanceNamespaces ...string) (*labels.Requirement, error) {
	instanceReq, err := GetLabelRequirements(common.AppK8sKeyInstance, selection.In, instanceNamespaces)
	if err != nil {
		return nil, errors.Wrap(err, "GetInstanceLabelRequirement: failed to generate requirement")
	}
	return instanceReq, nil
}

func GetComponentLabelRequirement(components ...string) (*labels.Requirement, error) {
	componentReq, err := GetLabelRequirements(common.AppK8sKeyComponent, selection.In, components)
	if err != nil {
		return nil, errors.Wrap(err, "GetComponentLabelRequirement: failed to generate requirement")
	}
	return componentReq, nil
}

func GetRbacTypeLabelRequirement(rbacTypes ...string) (*labels.Requirement, error) {
	componentReq, err := GetLabelRequirements(common.ArgoCDArgoprojKeyRBACType, selection.In, rbacTypes)
	if err != nil {
		return nil, errors.Wrap(err, "GetRbacTypeLabelRequirement: failed to generate requirement")
	}
	return componentReq, nil
}

func GetLabelRequirements(key string, op selection.Operator, vals []string) (*labels.Requirement, error) {
	newReq, err := labels.NewRequirement(key, op, vals)
	if err != nil {
		return nil, errors.Wrap(err, "GetLabelRequirements: failed to generate label selector")
	}

	return newReq, nil
}

func GetLabelSelector(reqs ...labels.Requirement) labels.Selector {
	return labels.NewSelector().Add(reqs...)
}
