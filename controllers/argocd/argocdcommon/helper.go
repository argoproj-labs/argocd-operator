package argocdcommon

import (
	"errors"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

type FieldToCompare struct {
	Existing    interface{}
	Desired     interface{}
	ExtraAction func()
}

// UpdateIfChanged accepts a slice of fields to be compared, along with a bool ptr. It compares all the provided fields, updating any fields and setting the bool ptr to true if a drift is detected
func UpdateIfChanged(ftc []FieldToCompare, changed *bool) {
	for _, field := range ftc {
		if util.IsPtr(field.Existing) && util.IsPtr(field.Desired) {
			if !reflect.DeepEqual(field.Existing, field.Desired) {
				reflect.ValueOf(field.Existing).Elem().Set(reflect.ValueOf(field.Desired).Elem())
				if field.ExtraAction != nil {
					field.ExtraAction()
				}
				*changed = true
			}
		}
	}
}

// PartialMatch accepts a slice of fields to be compared, along with a bool ptr. It compares all the provided fields and sets the bool to false if a drift is detected
func PartialMatch(ftc []FieldToCompare, match *bool) {
	for _, field := range ftc {
		if !reflect.DeepEqual(field.Existing, field.Desired) {
			*match = false
		}
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
