package argocdcommon

import (
	"errors"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

func UpdateIfChanged(existingVal, desiredVal interface{}, extraAction func(), changed *bool) {
	if !reflect.DeepEqual(existingVal, desiredVal) {
		reflect.ValueOf(existingVal).Elem().Set(reflect.ValueOf(desiredVal).Elem())
		if extraAction != nil {
			extraAction()
		}
		*changed = true
	}
}

// IsMergable returns error if any of the extraArgs is already part of the default command Arguments.
func IsMergable(extraArgs []string, cmd []string) error {
	if len(extraArgs) > 0 {
		for _, arg := range extraArgs {
			if len(arg) > 2 && arg[:2] == "--" {
				if ok := util.ContainsString(cmd, arg); ok {
					err := errors.New("duplicate argument error")
					return err
				}
			}
		}
	}
	return nil
}
