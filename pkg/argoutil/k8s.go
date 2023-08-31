package argoutil

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newEvent(meta metav1.ObjectMeta) *corev1.Event {
	event := &corev1.Event{}
	event.ObjectMeta.GenerateName = fmt.Sprintf("%s-", meta.Name)
	event.ObjectMeta.Labels = meta.Labels
	event.ObjectMeta.Namespace = meta.Namespace
	return event
}

// CreateEvent will create a new Kubernetes Event with the given action, message, reason and involved uid.
func CreateEvent(client client.Client, eventType, action, message, reason string, objectMeta metav1.ObjectMeta, typeMeta metav1.TypeMeta) error {
	event := newEvent(objectMeta)
	event.Action = action
	event.Type = eventType
	event.InvolvedObject = corev1.ObjectReference{
		Name:            objectMeta.Name,
		Namespace:       objectMeta.Namespace,
		UID:             objectMeta.UID,
		ResourceVersion: objectMeta.ResourceVersion,
		Kind:            typeMeta.Kind,
		APIVersion:      typeMeta.APIVersion,
	}
	event.Message = message
	event.Reason = reason
	event.CreationTimestamp = metav1.Now()
	event.FirstTimestamp = event.CreationTimestamp
	event.LastTimestamp = event.CreationTimestamp
	return client.Create(context.TODO(), event)
}
