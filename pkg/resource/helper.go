package resource

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "github.com/openshift/api/apps/v1"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConvertToRuntimeObjects converts a given set of client.Object resources into a slice of runtime.Objects
func ConvertToRuntimeObjects(objs ...client.Object) ([]runtime.Object, error) {
	runtimeObjs := []runtime.Object{}
	var conversionErr util.MultiError

	for _, obj := range objs {
		// Get the GVK (GroupVersionKind) of the client.Object
		gvk := obj.GetObjectKind().GroupVersionKind()

		sch := GetScheme()

		// Create a new empty runtime.Object with the same GVK
		newRuntimeObject, err := sch.New(gvk)
		conversionErr.Append(err)

		// DeepCopy the client.Object into the runtime.Object
		err = sch.Convert(obj, newRuntimeObject, nil)
		conversionErr.Append(err)

		runtimeObjs = append(runtimeObjs, newRuntimeObject)
	}

	return runtimeObjs, conversionErr.ErrOrNil()
}

func GetScheme() *runtime.Scheme {
	sOpts := func(s *runtime.Scheme) {
		argoproj.AddToScheme(s)
		monitoringv1.AddToScheme(s)
		routev1.Install(s)
		configv1.Install(s)
		templatev1.Install(s)
		appsv1.Install(s)
		oauthv1.Install(s)
	}
	return MakeScheme(sOpts)
}

type SchemeOpt func(*runtime.Scheme)

func MakeScheme(sOpts ...SchemeOpt) *runtime.Scheme {
	s := scheme.Scheme
	for _, opt := range sOpts {
		opt(s)
	}

	return s
}
