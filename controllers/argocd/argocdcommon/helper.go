package argocdcommon

import "reflect"

func UpdateIfChanged(existingVal, desiredVal interface{}, extraAction func(), changed *bool) {
	if !reflect.DeepEqual(existingVal, desiredVal) {
		reflect.ValueOf(existingVal).Elem().Set(reflect.ValueOf(desiredVal).Elem())
		if extraAction != nil {
			extraAction()
		}
		*changed = true
	}
}

func UpdateIfChangedSlice(existingVal, desiredVal interface{}, extraAction func(), changed *bool) (interface{}, interface{}) {
	if !reflect.DeepEqual(existingVal, desiredVal) {
		existingVal = desiredVal
		if extraAction != nil {
			extraAction()
		}
		*changed = true
	}

	return existingVal, desiredVal
}
