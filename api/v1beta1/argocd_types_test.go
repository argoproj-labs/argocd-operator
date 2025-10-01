package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-operator/common"
)

func Test_ArgoCD_ApplicationInstanceLabelKey(t *testing.T) {
	cr := &ArgoCD{}
	cr.Spec.ApplicationInstanceLabelKey = "my.corp/instance"
	assert.Equal(t, cr.ApplicationInstanceLabelKey(), "my.corp/instance")
	cr = &ArgoCD{}
	assert.Equal(t, cr.ApplicationInstanceLabelKey(), common.ArgoCDDefaultApplicationInstanceLabelKey)
}

func Test_ResourceTrackingMethodToString(t *testing.T) {
	testdata := []struct {
		rtm ResourceTrackingMethod
		str string
	}{
		{ResourceTrackingMethodLabel, stringResourceTrackingMethodLabel},
		{ResourceTrackingMethodAnnotation, stringResourceTrackingMethodAnnotation},
		{ResourceTrackingMethodAnnotationAndLabel, stringResourceTrackingMethodAnnotationAndLabel},
		{20, stringResourceTrackingMethodAnnotation},
	}
	for _, tt := range testdata {
		rtm := tt.rtm
		assert.Equal(t, tt.str, rtm.String())
	}
}

func Test_ParseResourceTrackingMethod(t *testing.T) {
	testdata := []struct {
		rtm ResourceTrackingMethod
		str string
	}{
		{ResourceTrackingMethodLabel, stringResourceTrackingMethodLabel},
		{ResourceTrackingMethodAnnotation, stringResourceTrackingMethodAnnotation},
		{ResourceTrackingMethodAnnotationAndLabel, stringResourceTrackingMethodAnnotationAndLabel},
		{ResourceTrackingMethodInvalid, "invalid"},
		{ResourceTrackingMethodAnnotation, ""},
	}
	for _, tt := range testdata {
		assert.Equal(t, tt.rtm, ParseResourceTrackingMethod(tt.str))
	}
}
