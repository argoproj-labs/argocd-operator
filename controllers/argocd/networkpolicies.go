package argocd

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var (
	TCPProtocol = func() *corev1.Protocol {
		tcpProtocol := corev1.ProtocolTCP
		return &tcpProtocol
	}()
)

const (
	// RedisIngressNetworkPolicy is the name of the network policy which controls Redis Ingress traffic
	RedisNetworkPolicy = "redis-network-policy"
	// RedisHAIngressNetworkPolicy is the name of the network policy which controls Redis HA Ingress traffic
	RedisHANetworkPolicy = "redis-ha-network-policy"
)

func (r *ReconcileArgoCD) ReconcileNetworkPolicies(cr *argoproj.ArgoCD) error {

	// Reconcile Redis network policy
	if err := r.ReconcileRedisNetworkPolicy(cr); err != nil {
		return err
	}

	// Reconcile Redis HA network policy
	if err := r.ReconcileRedisHANetworkPolicy(cr); err != nil {
		return err
	}

	return nil
}

// ReconcileRedisNetworkPolicy creates and reconciles network policy for Redis
func (r *ReconcileArgoCD) ReconcileRedisNetworkPolicy(cr *argoproj.ArgoCD) error {

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, RedisNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "redis"),
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "application-controller"),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "repo-server"),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "server"),
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 6379},
						},
					},
				},
			},
		},
	}

	// Check if the network policy already exists
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, RedisNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		modified := false
		if !reflect.DeepEqual(existing.Spec.PodSelector, networkPolicy.Spec.PodSelector) {
			existing.Spec.PodSelector = networkPolicy.Spec.PodSelector
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, networkPolicy.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = networkPolicy.Spec.PolicyTypes
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, networkPolicy.Spec.Ingress) {
			existing.Spec.Ingress = networkPolicy.Spec.Ingress
			modified = true
		}

		if modified {
			log.Info("Updating redis network policy", "namespace", networkPolicy.Namespace, "name", networkPolicy.Name)
			err := r.Client.Update(context.TODO(), existing)
			if err != nil {
				log.Error(err, "Failed to update redis network policy")
				return err
			}
		}

		// Nothing to do, NetworkPolicy already exists and not modified
		return nil

	}

	// Set the ArgoCD instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on redis network policy")
		return err
	}

	log.Info("Creating redis network policy", "namespace", networkPolicy.Namespace, "name", networkPolicy.Name)
	err := r.Client.Create(context.TODO(), networkPolicy)
	if err != nil {
		log.Error(err, "Failed to create redis network policy")
		return err
	}

	return nil

}

// ReconcileRedisHANetworkPolicy creates and reconciles network policy for Redis HA
func (r *ReconcileArgoCD) ReconcileRedisHANetworkPolicy(cr *argoproj.ArgoCD) error {

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, RedisHANetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "redis-ha-haproxy"),
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "application-controller"),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "repo-server"),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": fmt.Sprintf("%s-%s", cr.Name, "server"),
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 6379},
						},
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 26379},
						},
					},
				},
			},
		},
	}

	// Check if the network policy already exists
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, RedisHANetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		modified := false
		if !reflect.DeepEqual(existing.Spec.PodSelector, networkPolicy.Spec.PodSelector) {
			existing.Spec.PodSelector = networkPolicy.Spec.PodSelector
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, networkPolicy.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = networkPolicy.Spec.PolicyTypes
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, networkPolicy.Spec.Ingress) {
			existing.Spec.Ingress = networkPolicy.Spec.Ingress
			modified = true
		}

		if modified {
			log.Info("Updating redis ha network policy", "namespace", networkPolicy.Namespace, "name", networkPolicy.Name)
			err := r.Client.Update(context.TODO(), existing)
			if err != nil {
				log.Error(err, "Failed to update redis ha network policy")
				return err
			}
		}

		// Nothing to do, NetworkPolicy already exists and not modified
		return nil

	}

	// Set the ArgoCD instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on redis ha network policy")
		return err
	}

	log.Info("Creating redis ha network policy", "namespace", networkPolicy.Namespace, "name", networkPolicy.Name)
	err := r.Client.Create(context.TODO(), networkPolicy)
	if err != nil {
		log.Error(err, "Failed to create redis ha network policy")
		return err
	}

	return nil

}
