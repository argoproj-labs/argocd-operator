/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&NotificationsConfiguration{}, &NotificationsConfigurationList{})
}

//+kubebuilder:object:root=true

// NotificationsConfiguration is the Schema for the NotificationsConfiguration API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:resources={{NotificationsConfiguration,v1beta1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""}}
type NotificationsConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NotificationsConfigurationSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// NotificationsConfigurationList contains a list of NotificationsConfiguration
type NotificationsConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NotificationsConfiguration `json:"items"`
}

// NotificationsConfigurationSpec allows users to define the triggers, templates, services, context and
// subscriptions for the notifications
type NotificationsConfigurationSpec struct {
	// Triggers define the condition when the notification should be sent and list of templates required to generate the message
	// Recipients can subscribe to the trigger and specify the required message template and destination notification service.
	Triggers map[string]string `json:"triggers,omitempty"`
	// Templates are used to generate the notification template message
	Templates map[string]string `json:"templates,omitempty"`
	// Services are used to deliver message
	Services map[string]string `json:"services,omitempty"`
	// Subscriptions contain centrally managed global application subscriptions
	Subscriptions map[string]string `json:"subscriptions,omitempty"`
	// Context is used to define some shared context between all notification templates
	Context map[string]string `json:"context,omitempty"`
}
