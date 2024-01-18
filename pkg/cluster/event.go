package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// EventRequest objects contain all the required information to produce an event object in return
type EventRequest struct {
	InvolvedObjectMeta     metav1.ObjectMeta
	InvolvedObjectTypeMeta metav1.TypeMeta
	Action                 string
	Type                   string
	Message                string
	Reson                  string
	CreationTimestamp      metav1.Time
	FirstTimestamp         metav1.Time
	LastTimestamp          metav1.Time
	Instance               *argoproj.ArgoCD

	// array of functions to mutate event before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

func newEvent(objMeta metav1.ObjectMeta, typeMeta metav1.TypeMeta, eventType, action, message, reason string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: argoutil.NameWithSuffix(objMeta.Name, ""),
			Labels:       objMeta.Labels,
			Namespace:    objMeta.Namespace,
		},
		Action:  action,
		Type:    eventType,
		Message: message,
		Reason:  reason,
		InvolvedObject: corev1.ObjectReference{
			Name:            objMeta.Name,
			Namespace:       objMeta.Namespace,
			UID:             objMeta.UID,
			ResourceVersion: objMeta.ResourceVersion,
			Kind:            typeMeta.Kind,
			APIVersion:      typeMeta.APIVersion,
		},
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
	}
}

func RequestEvent(request EventRequest) (*corev1.Event, error) {
	var (
		mutationErr error
	)
	event := newEvent(request.InvolvedObjectMeta, request.InvolvedObjectTypeMeta, request.Type, request.Action, request.Message, request.Reson)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, event, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return event, fmt.Errorf("RequestEvent: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return event, nil
}

func CreateEvent(event *corev1.Event, client cntrlClient.Client) error {
	return client.Create(context.TODO(), event)
}
