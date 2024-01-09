package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

func GetAppControllerName(argoCDName string) string {
	return util.GenerateResourceName(argoCDName, AppControllerComponent)
}
