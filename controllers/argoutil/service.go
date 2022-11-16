package argoutil

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewService returns a new Service for the given instance.
func NewService(crName string, crNamespace string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: crNamespace,
			Labels:    LabelsForCluster(crName),
		},
	}
}

// newServiceWithName returns a new Service instance for the given instance using the given name.
func newServiceWithName(name string, component string, crName string, crNamespace string) *corev1.Service {
	svc := NewService(crName, crNamespace)
	svc.ObjectMeta.Name = name

	lbls := svc.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	lbls[common.ArgoCDKeyComponent] = component
	svc.ObjectMeta.Labels = lbls

	return svc
}

// NewServiceWithSuffix returns a new Service instance for the given instance using the given suffix.
func NewServiceWithSuffix(suffix string, component string, crName string, crNamespace string) *corev1.Service {
	return newServiceWithName(fmt.Sprintf("%s-%s", crName, suffix), component, crName, crNamespace)
}
