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
	"github.com/argoproj-labs/argocd-operator/common"
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
	// ArgoCDServerNetworkPolicy is the name of the network policy which controls Argo CD Server traffic
	ArgoCDServerNetworkPolicy = "server-network-policy"
	// ArgoCDApplicationControllerNetworkPolicy is the name of the network policy which controls Argo CD Application Controller traffic
	ArgoCDApplicationControllerNetworkPolicy = "application-controller-network-policy"
	// ArgoCDRepoServerNetworkPolicy is the name of the network policy which controls Argo CD Repo Server traffic
	ArgoCDRepoServerNetworkPolicy = "repo-server-network-policy"
	// ArgoCDNotificationsControllerNetworkPolicy is the name of the network policy which controls Argo CD Notifications Controller traffic
	ArgoCDNotificationsControllerNetworkPolicy = "notifications-controller-network-policy"
	// ArgoCDDexServerNetworkPolicy is the name of the network policy which controls Argo CD Dex Server traffic
	ArgoCDDexServerNetworkPolicy = "dex-server-network-policy"
	// ArgoCDApplicationSetControllerNetworkPolicy is the name of the network policy which controls Argo CD ApplicationSet Controller traffic
	ArgoCDApplicationSetControllerNetworkPolicy = "applicationset-controller-network-policy"
)

func (r *ReconcileArgoCD) ReconcileNetworkPolicies(cr *argoproj.ArgoCD) error {

	if !cr.Spec.NetworkPolicy.IsEnabled() {
		return r.deleteArgoCDNetworkPolicies(cr)
	}

	// Reconcile Redis network policy
	if err := r.ReconcileRedisNetworkPolicy(cr); err != nil {
		return err
	}

	// Reconcile Redis HA network policy
	if err := r.ReconcileRedisHANetworkPolicy(cr); err != nil {
		return err
	}

	// Reconcile Notifications Controller network policy
	if err := r.ReconcileNotificationsControllerNetworkPolicy(cr); err != nil {
		return err
	}

	// Reconcile Dex Server network policy
	if err := r.ReconcileDexServerNetworkPolicy(cr); err != nil {
		return err
	}

	// Reconcile ApplicationSet Controller network policy
	if err := r.ReconcileApplicationSetControllerNetworkPolicy(cr); err != nil {
		return err
	}

	if err := r.ReconcileArgoCDServerNetworkPolicy(cr); err != nil {
		return err
	}

	if err := r.ReconcileArgoCDApplicationControllerNetworkPolicy(cr); err != nil {
		return err
	}

	if err := r.ReconcileArgoCDRepoServerNetworkPolicy(cr); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) deleteArgoCDNetworkPolicies(cr *argoproj.ArgoCD) error {
	names := []string{
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDNotificationsControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDDexServerNetworkPolicy),
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationSetControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDServerNetworkPolicy),
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationControllerNetworkPolicy),
		fmt.Sprintf("%s-%s", cr.Name, ArgoCDRepoServerNetworkPolicy),
	}

	for _, name := range names {
		existing := &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: cr.Namespace,
			},
		}

		found, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
		if err != nil {
			return err
		}
		if !found {
			continue
		}

		argoutil.LogResourceDeletion(log, existing, "networkPolicy is disabled")
		if err := r.Delete(context.TODO(), existing); err != nil {
			return err
		}
	}

	return nil
}

// ReconcileDexServerNetworkPolicy creates and reconciles network policy for Dex Server
func (r *ReconcileArgoCD) ReconcileDexServerNetworkPolicy(cr *argoproj.ArgoCD) error {

	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDDexServerNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("dex-server", cr),
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
									"app.kubernetes.io/name": nameWithSuffix("server", cr),
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: common.ArgoCDDefaultDexHTTPPort},
						},
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: common.ArgoCDDefaultDexGRPCPort},
						},
					},
				},
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: common.ArgoCDDefaultDexMetricsPort},
						},
					},
				},
			},
		},
	}

	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDDexServerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if !UseDex(cr) {
		if npExists {
			argoutil.LogResourceDeletion(log, existing, "dex uninstallation has been requested")
			return r.Delete(context.TODO(), existing)
		}
		return nil
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update dex server network policy")
				return fmt.Errorf("failed to update dex server network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on dex server network policy")
		return fmt.Errorf("failed to set controller reference on dex server network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create dex server network policy")
		return fmt.Errorf("failed to create dex server network policy. error: %w", err)
	}

	return nil
}

// ReconcileApplicationSetControllerNetworkPolicy creates and reconciles network policy for ApplicationSet Controller
func (r *ReconcileArgoCD) ReconcileApplicationSetControllerNetworkPolicy(cr *argoproj.ArgoCD) error {

	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationSetControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("applicationset-controller", cr),
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 7000},
						},
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
						},
					},
				},
			},
		},
	}

	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationSetControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.IsEnabled() {
		if npExists {
			argoutil.LogResourceDeletion(log, existing, "application set not enabled")
			return r.Delete(context.TODO(), existing)
		}
		return nil
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update applicationset controller network policy")
				return fmt.Errorf("failed to update applicationset controller network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on applicationset controller network policy")
		return fmt.Errorf("failed to set controller reference on applicationset controller network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create applicationset controller network policy")
		return fmt.Errorf("failed to create applicationset controller network policy. error: %w", err)
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
					"app.kubernetes.io/name": nameWithSuffix("redis", cr),
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
									"app.kubernetes.io/name": nameWithSuffix("application-controller", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("repo-server", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("server", cr),
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

	if cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Principal != nil && cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		networkPolicy.Spec.Ingress[0].From = append(networkPolicy.Spec.Ingress[0].From, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("agent-principal", cr),
				},
			},
		})
	}

	if cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Agent != nil && cr.Spec.ArgoCDAgent.Agent.IsEnabled() {
		networkPolicy.Spec.Ingress[0].From = append(networkPolicy.Spec.Ingress[0].From, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("agent-agent", cr),
				},
			},
		})
	}

	// Check if the network policy already exists
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, RedisNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if npExists {

		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, networkPolicy.Spec.PodSelector) {
			existing.Spec.PodSelector = networkPolicy.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, networkPolicy.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = networkPolicy.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, networkPolicy.Spec.Ingress) {
			existing.Spec.Ingress = networkPolicy.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			err := r.Update(context.TODO(), existing)
			if err != nil {
				log.Error(err, "Failed to update redis network policy")
				return fmt.Errorf("failed to update redis network policy. error: %w", err)
			}
		}

		// Nothing to do, NetworkPolicy already exists and not modified
		return nil

	}

	// Set the ArgoCD instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on redis network policy")
		return fmt.Errorf("failed to set controller reference on redis network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, networkPolicy)
	if err := r.Create(context.TODO(), networkPolicy); err != nil {
		log.Error(err, "Failed to create redis network policy")
		return fmt.Errorf("failed to create redis network policy. error: %w", err)
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
					"app.kubernetes.io/name": nameWithSuffix("redis-ha-haproxy", cr),
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
									"app.kubernetes.io/name": nameWithSuffix("application-controller", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("repo-server", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("server", cr),
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

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}
	if npExists {

		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, networkPolicy.Spec.PodSelector) {
			existing.Spec.PodSelector = networkPolicy.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, networkPolicy.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = networkPolicy.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, networkPolicy.Spec.Ingress) {
			existing.Spec.Ingress = networkPolicy.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			err := r.Update(context.TODO(), existing)
			if err != nil {
				log.Error(err, "Failed to update redis ha network policy")
				return fmt.Errorf("failed to update redis ha network policy. error: %w", err)
			}
		}

		// Nothing to do, NetworkPolicy already exists and not modified
		return nil

	}

	// Set the ArgoCD instance as the owner and controller
	if err := controllerutil.SetControllerReference(cr, networkPolicy, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on redis ha network policy")
		return fmt.Errorf("failed to set controller reference on redis ha network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, networkPolicy)

	if err := r.Create(context.TODO(), networkPolicy); err != nil {
		log.Error(err, "Failed to create redis ha network policy")
		return fmt.Errorf("failed to create redis ha network policy. error: %w", err)
	}

	return nil

}

// ReconcileNotificationsControllerNetworkPolicy creates and reconciles network policy for Notifications Controller
func (r *ReconcileArgoCD) ReconcileNotificationsControllerNetworkPolicy(cr *argoproj.ArgoCD) error {

	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDNotificationsControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("notifications-controller", cr),
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 9001},
						},
					},
				},
			},
		},
	}

	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDNotificationsControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if !isNotificationsEnabled(cr) {
		if npExists {
			argoutil.LogResourceDeletion(log, existing, "notifications are disabled")
			return r.Delete(context.TODO(), existing)
		}
		return nil
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update notifications controller network policy")
				return fmt.Errorf("failed to update notifications controller network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on notifications controller network policy")
		return fmt.Errorf("failed to set controller reference on notifications controller network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create notifications controller network policy")
		return fmt.Errorf("failed to create notifications controller network policy. error: %w", err)
	}

	return nil
}

func (r *ReconcileArgoCD) ReconcileArgoCDServerNetworkPolicy(cr *argoproj.ArgoCD) error {
	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDServerNetworkPolicy),
			Namespace: cr.Namespace,
		},

		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("server", cr),
				},
			},

			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{},
			},
		},
	}

	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDServerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update server network policy")
				return fmt.Errorf("failed to update server network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on argocd server network policy")
		return fmt.Errorf("failed to set controller reference on argocd server network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create server network policy")
		return fmt.Errorf("failed to create server network policy. error: %w", err)
	}

	return nil
}

func (r *ReconcileArgoCD) ReconcileArgoCDApplicationControllerNetworkPolicy(cr *argoproj.ArgoCD) error {
	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("application-controller", cr),
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8082},
						},
					},
				},
			},
		},
	}

	// Check if the network policy already exists
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDApplicationControllerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	// Check if the network policy already exists
	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update application controller network policy")
				return fmt.Errorf("failed to update application controller network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on argocd application controller network policy")
		return fmt.Errorf("failed to set controller reference on argocd application controller network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create application controller network policy")
		return fmt.Errorf("failed to create application controller network policy. error: %w", err)
	}

	return nil
}

func (r *ReconcileArgoCD) ReconcileArgoCDRepoServerNetworkPolicy(cr *argoproj.ArgoCD) error {
	desired := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDRepoServerNetworkPolicy),
			Namespace: cr.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": nameWithSuffix("repo-server", cr),
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
									"app.kubernetes.io/name": nameWithSuffix("application-controller", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("server", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("notifications-controller", cr),
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app.kubernetes.io/name": nameWithSuffix("applicationset-controller", cr),
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8081},
						},
					},
				},
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: TCPProtocol,
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 8084},
						},
					},
				},
			},
		},
	}

	// Check if the network policy already exists
	existing := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cr.Name, ArgoCDRepoServerNetworkPolicy),
			Namespace: cr.Namespace,
		},
	}

	// Check if the network policy already exists
	npExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing)
	if err != nil {
		return err
	}

	if npExists {
		modified := false
		explanation := ""
		if !reflect.DeepEqual(existing.Spec.PodSelector, desired.Spec.PodSelector) {
			existing.Spec.PodSelector = desired.Spec.PodSelector
			explanation = "pod selector"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.PolicyTypes, desired.Spec.PolicyTypes) {
			existing.Spec.PolicyTypes = desired.Spec.PolicyTypes
			if modified {
				explanation += ", "
			}
			explanation += "policy types"
			modified = true
		}
		if !reflect.DeepEqual(existing.Spec.Ingress, desired.Spec.Ingress) {
			existing.Spec.Ingress = desired.Spec.Ingress
			if modified {
				explanation += ", "
			}
			explanation += "ingress rules"
			modified = true
		}

		if modified {
			argoutil.LogResourceUpdate(log, existing, "updating", explanation)
			if err := r.Update(context.TODO(), existing); err != nil {
				log.Error(err, "Failed to update repo server network policy")
				return fmt.Errorf("failed to update repo server network policy. error: %w", err)
			}
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		log.Error(err, "Failed to set controller reference on argocd repo server network policy")
		return fmt.Errorf("failed to set controller reference on argocd repo server network policy. error: %w", err)
	}

	argoutil.LogResourceCreation(log, desired)
	if err := r.Create(context.TODO(), desired); err != nil {
		log.Error(err, "Failed to create repo server network policy")
		return fmt.Errorf("failed to create repo server network policy. error: %w", err)
	}

	return nil
}
