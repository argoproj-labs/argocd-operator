package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

func GetAppControllerName(argoCDName string) string {
	return argoutil.GenerateResourceName(argoCDName, AppControllerComponent)
}
