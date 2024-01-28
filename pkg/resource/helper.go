package resource

import (
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ConvertToRuntimeObjects(objs ...client.Object) ([]runtime.Object, error) {
	runtimeObjs := []runtime.Object{}
	var conversionErr util.MultiError

	for _, obj := range objs {
		// Get the GVK (GroupVersionKind) of the client.Object
		gvk := obj.GetObjectKind().GroupVersionKind()

		// Create a new empty runtime.Object with the same GVK
		newRuntimeObject, err := scheme.Scheme.New(gvk)
		conversionErr.Append(err)

		// DeepCopy the client.Object into the runtime.Object
		err = scheme.Scheme.Convert(obj, newRuntimeObject, nil)
		conversionErr.Append(err)

		runtimeObjs = append(runtimeObjs, newRuntimeObject)
	}

	return runtimeObjs, conversionErr.ErrOrNil()
}
