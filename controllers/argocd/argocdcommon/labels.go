package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

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

func GetComponentLabelRequirement(component string) (*labels.Requirement, error) {
	componentReq, err := GetLabelRequirements(common.AppK8sKeyComponent, selection.Equals, []string{component})
	if err != nil {
		return nil, errors.Wrap(err, "getRbacTypeReq: failed to generate requirement")
	}
	return componentReq, nil
}

func GetResourceMgmtResourceLabelSelector(componentReq *labels.Requirement) (labels.Selector, error) {
	resMgmtReq, err := GetLabelRequirements(common.ArgoCDArgoprojKeyRBACType, selection.Equals, []string{common.ArgoCDRBACTypeResourceMananagement})
	if err != nil {
		return nil, errors.Wrap(err, "getResourceMgmtResourceLabelSelector: failed to generate requirement")
	}

	resMgmtLs := GetLabelSelector(*resMgmtReq, *componentReq)
	return resMgmtLs, nil
}

func GetAppMgmtResourceLabelSelector(componentReq *labels.Requirement) (labels.Selector, error) {
	appMgmtReq, err := GetLabelRequirements(common.ArgoCDArgoprojKeyRBACType, selection.Equals, []string{common.ArgoCDRBACTypeAppManagement})
	if err != nil {
		return nil, errors.Wrap(err, "getAppMgmtResourceLabelSelector: failed to generate requirement")
	}

	appMgmtLs := GetLabelSelector(*appMgmtReq, *componentReq)
	return appMgmtLs, nil
}

func GetAppsetMgmtResourceLabelSelector(componentReq *labels.Requirement) (labels.Selector, error) {
	resMgmtReq, err := GetLabelRequirements(common.ArgoCDArgoprojKeyRBACType, selection.Equals, []string{common.ArgoCDRBACTypeAppSetManagement})
	if err != nil {
		return nil, errors.Wrap(err, "GetAppsetMgmtResourceLabelSelector: failed to generate requirement")
	}

	resMgmtLs := GetLabelSelector(*resMgmtReq, *componentReq)
	return resMgmtLs, nil
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
