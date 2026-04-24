package argocd

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	DefaultImageUpdaterImage      = "quay.io/argoprojlabs/argocd-image-updater"
	DefaultImageUpdaterTag        = "v1.1.1"
	ArgocdImageUpdaterConfigCM    = "argocd-image-updater-config"
	ArgocdImageUpdaterSSHConfigCM = "argocd-image-updater-ssh-config"
	ArgocdImageUpdaterSecret      = "argocd-image-updater-secret" // #nosec G101
)

func (r *ReconcileArgoCD) reconcileImageUpdaterController(cr *argoproj.ArgoCD) error {
	if cr.Spec.ImageUpdater.Enabled {
		return r.reconcileImageUpdaterControllerEnabled(cr)
	}
	return r.reconcileImageUpdaterControllerDisabled(cr)
}

func (r *ReconcileArgoCD) reconcileImageUpdaterControllerEnabled(cr *argoproj.ArgoCD) error {
	log.Info("reconciling Image Updater service account")
	sa, err := r.reconcileImageUpdaterServiceAccount(cr)
	if err != nil {
		return err
	}

	// Determine the watch scope from IMAGE_UPDATER_WATCH_NAMESPACES before creating roles,
	// because the required role set depends on the mode.
	// Three modes are supported:
	//   - Not set or empty: namespace-scoped (Option 1). Controller watches only its own namespace.
	//     Single role with all rules in cr.Namespace (base + manager rules combined).
	//   - "*": cluster-scoped (Option 3). Controller watches all namespaces.
	//     Base role in cr.Namespace + ClusterRole with manager rules. Requires cluster-config namespace.
	//   - "ns1,ns2,...": watches specific namespaces.
	//     Base role in cr.Namespace + manager Role in each listed namespace.
	watchNamespaces := ""
	for _, env := range cr.Spec.ImageUpdater.Env {
		if env.Name == "IMAGE_UPDATER_WATCH_NAMESPACES" {
			watchNamespaces = strings.TrimSpace(env.Value)
			break
		}
	}

	switch watchNamespaces {
	case "*":
		if !argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace) {
			return fmt.Errorf("IMAGE_UPDATER_WATCH_NAMESPACES=\"*\" can only be configured in cluster scope")
		}
		// Base role (configmaps, secrets, leases, events) in cr.Namespace.
		log.Info("reconciling Image Updater role")
		role, err := r.reconcileImageUpdaterRole(cr, policyRuleForRoleForImageUpdaterController())
		if err != nil {
			return err
		}
		if sa != nil && role != nil {
			log.Info("reconciling Image Updater role binding")
			if err := r.reconcileImageUpdaterRoleBinding(cr, role, sa); err != nil {
				return err
			}
		}
		// ClusterRole for cluster-wide manager rules (imageupdaters, applications, events).
		log.Info("using cluster-scoped installation for Image Updater")
		log.Info("reconciling Image Updater cluster role")
		clusterRole, err := r.reconcileImageUpdaterClusterRole(cr)
		if err != nil {
			return err
		}
		if sa != nil && clusterRole != nil {
			log.Info("reconciling Image Updater cluster role binding")
			if err := r.reconcileImageUpdaterClusterRoleBinding(cr, clusterRole, sa); err != nil {
				return err
			}
		}
	case "":
		// Namespace-scoped: both base and manager rules apply to cr.Namespace, so combine them into a single role.
		log.Info("using namespace-scoped installation for Image Updater", "namespace", cr.Namespace)
		log.Info("reconciling Image Updater role", "namespace", cr.Namespace)
		allRules := append(policyRuleForRoleForImageUpdaterController(), policyRuleForRoleManagerRoleForImageUpdaterController()...)
		role, err := r.reconcileImageUpdaterRole(cr, allRules)
		if err != nil {
			return err
		}
		if sa != nil && role != nil {
			log.Info("reconciling Image Updater role binding", "namespace", cr.Namespace)
			if err := r.reconcileImageUpdaterRoleBinding(cr, role, sa); err != nil {
				return err
			}
		}
	default:
		// Comma-separated list: base role in cr.Namespace + manager Role in each listed namespace.
		log.Info("reconciling Image Updater role")
		role, err := r.reconcileImageUpdaterRole(cr, policyRuleForRoleForImageUpdaterController())
		if err != nil {
			return err
		}
		if sa != nil && role != nil {
			log.Info("reconciling Image Updater role binding")
			if err := r.reconcileImageUpdaterRoleBinding(cr, role, sa); err != nil {
				return err
			}
		}
		for _, ns := range strings.Split(watchNamespaces, ",") {
			ns = strings.TrimSpace(ns)
			if ns == "" {
				continue
			}
			log.Info("reconciling Image Updater manager role", "namespace", ns)
			nsRole, err := r.reconcileImageUpdaterRoleForNamespace(ns, cr, policyRuleForRoleManagerRoleForImageUpdaterController())
			if err != nil {
				return err
			}
			if sa != nil && nsRole != nil {
				log.Info("reconciling Image Updater manager role binding", "namespace", ns)
				if err := r.reconcileImageUpdaterRoleBindingForNamespace(ns, cr, nsRole, sa); err != nil {
					return err
				}
			}
		}
	}

	log.Info("reconciling Image Updater secret")
	if err := r.reconcileImageUpdaterSecret(cr); err != nil {
		return err
	}

	imageUpdaterConfigMaps := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ArgocdImageUpdaterConfigCM,
				Namespace: cr.Namespace,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ArgocdImageUpdaterSSHConfigCM,
				Namespace: cr.Namespace,
			},
		},
	}

	for _, cm := range imageUpdaterConfigMaps {
		log.Info("reconciling Image Updater configmap", "name", cm.Name)
		if err := r.reconcileImageUpdaterConfigMap(cr, cm); err != nil {
			return err
		}
	}

	if sa != nil {
		log.Info("reconciling Image Updater deployment")
		if err := r.reconcileImageUpdaterDeployment(cr, sa); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterControllerDisabled(cr *argoproj.ArgoCD) error {
	// During deletion, we need to pass non-nil objects to reconcilers.
	// We can fetch them here. If they are not found, it's ok, maybe they are already deleted.
	// The individual reconcile functions will handle the 'NotFound' error when fetching for updates.
	sa := &corev1.ServiceAccount{}
	saName := getServiceAccountName(cr.Name, common.ArgoCDImageUpdaterControllerComponent)
	_ = argoutil.FetchObject(r.Client, cr.Namespace, saName, sa)
	if sa.Name == "" { // if fetch failed
		sa.Name = saName
		sa.Namespace = cr.Namespace
	}

	role := &rbacv1.Role{}
	roleName := generateResourceName(common.ArgoCDImageUpdaterControllerComponent, cr)
	_ = argoutil.FetchObject(r.Client, cr.Namespace, roleName, role)
	if role.Name == "" {
		role.Name = roleName
	}

	clusterRole := &rbacv1.ClusterRole{}
	clusterRoleName := GenerateUniqueResourceName(common.ArgoCDImageUpdaterControllerComponent, cr)
	_ = argoutil.FetchObject(r.Client, "", clusterRoleName, clusterRole)
	if clusterRole.Name == "" {
		clusterRole.Name = clusterRoleName
	}

	log.Info("deleting Image Updater deployment")
	if err := r.reconcileImageUpdaterDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("deleting Image Updater role binding")
	if err := r.reconcileImageUpdaterRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("deleting Image Updater cluster role binding")
	if err := r.reconcileImageUpdaterClusterRoleBinding(cr, clusterRole, sa); err != nil {
		return err
	}

	log.Info("deleting Image Updater service account")
	if _, err := r.reconcileImageUpdaterServiceAccount(cr); err != nil {
		return err
	}

	// reconcileImageUpdaterRole deletes the role in cr.Namespace, which covers both the
	// base role (Option 1/3) and the manager role (Option 1) since they share the same name.
	log.Info("deleting Image Updater role")
	if _, err := r.reconcileImageUpdaterRole(cr, policyRuleForRoleForImageUpdaterController()); err != nil {
		return err
	}

	log.Info("deleting Image Updater cluster role")
	if _, err := r.reconcileImageUpdaterClusterRole(cr); err != nil {
		return err
	}

	// For a comma-separated IMAGE_UPDATER_WATCH_NAMESPACES, manager roles were created in
	// each listed namespace. Delete them here. Option 1 ("") is handled above (same role name
	// in cr.Namespace); Option 3 ("*") used a ClusterRole, also handled above.
	watchNamespaces := ""
	for _, env := range cr.Spec.ImageUpdater.Env {
		if env.Name == "IMAGE_UPDATER_WATCH_NAMESPACES" {
			watchNamespaces = strings.TrimSpace(env.Value)
			break
		}
	}
	if watchNamespaces != "" && watchNamespaces != "*" {
		for _, ns := range strings.Split(watchNamespaces, ",") {
			ns = strings.TrimSpace(ns)
			if ns == "" {
				continue
			}
			log.Info("deleting Image Updater manager role", "namespace", ns)
			nsRole, err := r.reconcileImageUpdaterRoleForNamespace(ns, cr, policyRuleForRoleManagerRoleForImageUpdaterController())
			if err != nil {
				return err
			}
			if nsRole == nil {
				// Role was not found — fetch a stub so the RoleBinding deletion can proceed.
				nsRole = &rbacv1.Role{}
				nsRole.Name = getRoleNameForApplicationSourceNamespaces(ns, cr)
			}
			log.Info("deleting Image Updater manager role binding", "namespace", ns)
			if err := r.reconcileImageUpdaterRoleBindingForNamespace(ns, cr, nsRole, sa); err != nil {
				return err
			}
		}
	}

	log.Info("deleting Image Updater secret")
	if err := r.reconcileImageUpdaterSecret(cr); err != nil {
		return err
	}

	imageUpdaterConfigMaps := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ArgocdImageUpdaterConfigCM,
				Namespace: cr.Namespace,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ArgocdImageUpdaterSSHConfigCM,
				Namespace: cr.Namespace,
			},
		},
	}

	for _, cm := range imageUpdaterConfigMaps {
		log.Info("deleting Image Updater configmap", "name", cm.Name)
		if err := r.reconcileImageUpdaterConfigMap(cr, cm); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterServiceAccount(cr *argoproj.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName(common.ArgoCDImageUpdaterControllerComponent, cr)

	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the serviceAccount associated with %s : %s", sa.Name, err)
		}

		// SA doesn't exist and shouldn't, nothing to do here
		if !cr.Spec.ImageUpdater.Enabled {
			return nil, nil
		}

		// SA doesn't exist but should, so it should be created
		if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
			return nil, err
		}

		argoutil.LogResourceCreation(log, sa)
		err := r.Create(context.TODO(), sa)
		if err != nil {
			return nil, err
		}
	}

	// SA exists but shouldn't, so it should be deleted
	if !cr.Spec.ImageUpdater.Enabled {
		argoutil.LogResourceDeletion(log, sa, "image updater is disabled")
		return nil, r.Delete(context.TODO(), sa)
	}

	return sa, nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterRole(cr *argoproj.ArgoCD, policyRules []rbacv1.PolicyRule) (*rbacv1.Role, error) {
	desiredRole := newRole(common.ArgoCDImageUpdaterControllerComponent, policyRules, cr)
	role, err := r.reconcileRoleHelper(cr, desiredRole)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return role.(*rbacv1.Role), nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterRoleBinding(cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {
	desiredRoleBinding := newRoleBindingWithname(common.ArgoCDImageUpdaterControllerComponent, cr)
	desiredRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	desiredRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind: rbacv1.ServiceAccountKind,
			Name: sa.Name,
		},
	}

	return r.reconcileRoleBindingHelper(cr, desiredRoleBinding)
}

func (r *ReconcileArgoCD) reconcileImageUpdaterRoleForNamespace(namespace string, cr *argoproj.ArgoCD, policyRules []rbacv1.PolicyRule) (*rbacv1.Role, error) {
	desiredRole := newRoleForApplicationSourceNamespaces(namespace, policyRules, cr)
	role, err := r.reconcileRoleHelper(cr, desiredRole)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return role.(*rbacv1.Role), nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterRoleBindingForNamespace(namespace string, cr *argoproj.ArgoCD, role *rbacv1.Role, sa *corev1.ServiceAccount) error {
	desiredRoleBinding := newRoleBindingForSupportNamespaces(cr, namespace)
	desiredRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}
	// The ServiceAccount lives in cr.Namespace; specify its namespace explicitly
	// because the RoleBinding is in a different namespace.
	desiredRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
	return r.reconcileRoleBindingHelper(cr, desiredRoleBinding)
}

func (r *ReconcileArgoCD) reconcileImageUpdaterClusterRole(cr *argoproj.ArgoCD) (*rbacv1.ClusterRole, error) {
	policyRules := policyRuleForRoleManagerRoleForImageUpdaterController()
	desiredClusterRole := newClusterRole(common.ArgoCDImageUpdaterControllerComponent, policyRules, cr)
	clusterRole, err := r.reconcileRoleHelper(cr, desiredClusterRole)
	if err != nil {
		return nil, err
	}
	if clusterRole == nil {
		return nil, nil
	}
	return clusterRole.(*rbacv1.ClusterRole), nil
}

func (r *ReconcileArgoCD) reconcileImageUpdaterClusterRoleBinding(cr *argoproj.ArgoCD, clusterRole *rbacv1.ClusterRole, sa *corev1.ServiceAccount) error {

	desiredClusterRoleBinding := newClusterRoleBindingWithname(common.ArgoCDImageUpdaterControllerComponent, cr)
	desiredClusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}

	desiredClusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	return r.reconcileRoleBindingHelper(cr, desiredClusterRoleBinding)
}

// reconcileImageUpdaterSecret only creates/deletes the argocd-image-updater-secret based on whether image updater is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the secret itself
func (r *ReconcileArgoCD) reconcileImageUpdaterSecret(cr *argoproj.ArgoCD) error {
	desiredSecret := argoutil.NewSecretWithName(cr, ArgocdImageUpdaterSecret)
	return r.reconcileSecretConfigMapHelper(cr, desiredSecret)
}

// reconcileImageUpdaterConfigMap only creates/deletes the argocd-image-updater-config, argocd-image-updater-ssh-config
// based on whether image updater is enabled/disabled in the CR
// It does not reconcile/overwrite any fields or information in the configmap itself
func (r *ReconcileArgoCD) reconcileImageUpdaterConfigMap(cr *argoproj.ArgoCD, desiredConfigMap *corev1.ConfigMap) error {
	argoutil.AddTrackedByOperatorLabel(&desiredConfigMap.ObjectMeta)
	return r.reconcileSecretConfigMapHelper(cr, desiredConfigMap)
}

func (r *ReconcileArgoCD) reconcileImageUpdaterDeployment(cr *argoproj.ArgoCD, sa *corev1.ServiceAccount) error {

	desiredDeployment := newDeploymentWithSuffix(common.ArgoCDImageUpdaterControllerComponent, "controller", cr)

	desiredDeployment.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}

	imageUpdaterEnv := cr.Spec.ImageUpdater.Env
	// Let user specify their own environment first
	imageUpdaterEnv = argoutil.EnvMerge(imageUpdaterEnv, proxyEnvVars(), false)

	podSpec := &desiredDeployment.Spec.Template.Spec
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)
	podSpec.ServiceAccountName = sa.Name
	podSpec.Volumes = []corev1.Volume{
		{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDTLSCertsConfigMapName,
					},
				},
			},
		},
		{
			Name: "image-updater-conf",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ArgocdImageUpdaterConfigCM,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "registries.conf",
							Path: "registries.conf",
						},
						{
							Key:  "git.commit-message-template",
							Path: "commit.template",
						},
					},
				},
			},
		},
		{
			Name: "ssh-known-hosts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "argocd-ssh-known-hosts-cm",
					},
				},
			},
		},
		{
			Name: "ssh-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					Optional: boolPtr(true),
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ArgocdImageUpdaterSSHConfigCM,
					},
				},
			},
		},
		{
			Name: "ssh-signing-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "ssh-git-creds",
					Optional:   boolPtr(true),
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	image := os.Getenv(common.ArgoCDImageUpdaterImageEnvName)
	if image == "" {
		image = argoutil.CombineImageTag(DefaultImageUpdaterImage, DefaultImageUpdaterTag)
	}

	podSpec.Containers = []corev1.Container{{
		Command:         []string{"/manager"},
		Args:            []string{"run"},
		Image:           image,
		ImagePullPolicy: argoutil.GetImagePullPolicy(cr.Spec.ImagePullPolicy),
		Name:            common.ArgoCDImageUpdaterControllerComponent,
		Env:             imageUpdaterEnv,
		Resources:       getImageUpdaterResources(cr),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{
						IntVal: int32(8081),
					},
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{
						IntVal: int32(8081),
					},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		SecurityContext: argoutil.DefaultSecurityContext(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "tls-certs",
				MountPath: "/app/config/tls",
			},
			{
				Name:      "image-updater-conf",
				MountPath: "/app/config",
			},
			{
				Name:      "ssh-known-hosts",
				MountPath: "/app/config/ssh",
			},
			{
				Name:      "ssh-config",
				MountPath: "/app/.ssh",
			},
			{
				Name:      "tmp",
				MountPath: "/tmp",
			},
			{
				Name:      "ssh-signing-key",
				MountPath: "/app/ssh-keys/id_rsa",
				ReadOnly:  true,
				SubPath:   "sshPrivateKey",
			},
		},
	}}

	return r.reconcileDeploymentHelper(cr, desiredDeployment, "image updater", cr.Spec.ImageUpdater.Enabled)
}

// ========================= Helpers =========================

func (r *ReconcileArgoCD) reconcileRoleHelper(cr *argoproj.ArgoCD, desiredRole client.Object) (client.Object, error) {
	existingRole := reflect.New(reflect.TypeOf(desiredRole).Elem()).Interface().(client.Object)
	namespace := cr.Namespace

	switch r := desiredRole.(type) {
	case *rbacv1.Role:
		if ns := desiredRole.GetNamespace(); ns != "" {
			namespace = ns
		}
	case *rbacv1.ClusterRole:
		namespace = ""
	default:
		return nil, fmt.Errorf("unsupported type for reconcileRoleResource, got %T", r)
	}

	if err := argoutil.FetchObject(r.Client, namespace, desiredRole.GetName(), existingRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get the role associated with %s : %s", desiredRole.GetName(), err)
		}

		// role does not exist and shouldn't, nothing to do here
		if !cr.Spec.ImageUpdater.Enabled {
			return nil, nil
		}

		// role does not exist but should, so it should be created.
		// Owner references are only set for objects in the same namespace as cr;
		// cross-namespace owner references are forbidden by Kubernetes.
		if _, ok := desiredRole.(*rbacv1.Role); ok && desiredRole.GetNamespace() == cr.Namespace {
			if err := controllerutil.SetControllerReference(cr, desiredRole, r.Scheme); err != nil {
				return nil, err
			}
		}

		argoutil.LogResourceCreation(log, desiredRole)
		if err := r.Create(context.TODO(), desiredRole); err != nil {
			return nil, err
		}
		return desiredRole, nil
	}

	// role exists but shouldn't, so it should be deleted
	if !cr.Spec.ImageUpdater.Enabled {
		argoutil.LogResourceDeletion(log, existingRole, "image updater is disabled")
		return nil, r.Delete(context.TODO(), existingRole)
	}

	// role exists and should. Reconcile if changed
	desiredRules := getRulesFromRole(desiredRole)
	existingRules := getRulesFromRole(existingRole)
	if !reflect.DeepEqual(existingRules, desiredRules) {
		setRulesOnRole(existingRole, desiredRules)
		if _, ok := existingRole.(*rbacv1.Role); ok && existingRole.GetNamespace() == cr.Namespace {
			if err := controllerutil.SetControllerReference(cr, existingRole, r.Scheme); err != nil {
				return nil, err
			}
		}
		argoutil.LogResourceUpdate(log, existingRole, "updating policy rules")
		return existingRole, r.Update(context.TODO(), existingRole)
	}

	return desiredRole, nil
}

func getRulesFromRole(role client.Object) []rbacv1.PolicyRule {
	switch r := role.(type) {
	case *rbacv1.Role:
		return r.Rules
	case *rbacv1.ClusterRole:
		return r.Rules
	}
	return nil
}

func setRulesOnRole(role client.Object, rules []rbacv1.PolicyRule) {
	switch r := role.(type) {
	case *rbacv1.Role:
		r.Rules = rules
	case *rbacv1.ClusterRole:
		r.Rules = rules
	}
}

func (r *ReconcileArgoCD) reconcileRoleBindingHelper(cr *argoproj.ArgoCD, desiredRoleBinding client.Object) error {
	existingRoleBinding := reflect.New(reflect.TypeOf(desiredRoleBinding).Elem()).Interface().(client.Object)

	switch desiredRoleBinding.(type) {
	case *rbacv1.RoleBinding, *rbacv1.ClusterRoleBinding:
	default:
		return fmt.Errorf("unsupported type for reconcileRoleBindingResource resource, got %T", desiredRoleBinding)
	}

	namespace := cr.Namespace
	if _, ok := desiredRoleBinding.(*rbacv1.ClusterRoleBinding); ok {
		namespace = ""
	} else if ns := desiredRoleBinding.GetNamespace(); ns != "" {
		namespace = ns
	}

	// fetch existing rolebinding by name
	if err := r.Get(context.TODO(), types.NamespacedName{Name: desiredRoleBinding.GetName(), Namespace: namespace}, existingRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", desiredRoleBinding.GetName(), err)
		}

		// roleBinding does not exist and shouldn't, nothing to do here
		if !cr.Spec.ImageUpdater.Enabled {
			return nil
		}

		// roleBinding does not exist but should, so it should be created.
		// Owner references are only set for objects in the same namespace as cr;
		// cross-namespace owner references are forbidden by Kubernetes.
		if _, ok := desiredRoleBinding.(*rbacv1.RoleBinding); ok && desiredRoleBinding.GetNamespace() == cr.Namespace {
			if err := controllerutil.SetControllerReference(cr, desiredRoleBinding, r.Scheme); err != nil {
				return err
			}
		}

		argoutil.LogResourceCreation(log, desiredRoleBinding)
		return r.Create(context.TODO(), desiredRoleBinding)
	}

	// roleBinding exists but shouldn't, so it should be deleted
	if !cr.Spec.ImageUpdater.Enabled {
		argoutil.LogResourceDeletion(log, existingRoleBinding, "image updater is disabled")
		return r.Delete(context.TODO(), existingRoleBinding)
	}

	// roleBinding exists and should. Reconcile roleBinding if changed
	if !reflect.DeepEqual(getRoleRefFromRoleBinding(existingRoleBinding), getRoleRefFromRoleBinding(desiredRoleBinding)) {
		argoutil.LogResourceDeletion(log, existingRoleBinding, "roleref changed, deleting rolebinding in order to recreate it")
		if err := r.Delete(context.TODO(), existingRoleBinding); err != nil {
			return err
		}
	} else if !reflect.DeepEqual(getSubjectsFromRoleBinding(existingRoleBinding), getSubjectsFromRoleBinding(desiredRoleBinding)) {
		setSubjectsOnRoleBinding(existingRoleBinding, getSubjectsFromRoleBinding(desiredRoleBinding))
		if _, ok := existingRoleBinding.(*rbacv1.RoleBinding); ok && existingRoleBinding.GetNamespace() == cr.Namespace {
			if err := controllerutil.SetControllerReference(cr, existingRoleBinding, r.Scheme); err != nil {
				return err
			}
		}
		argoutil.LogResourceUpdate(log, existingRoleBinding, "updating subjects")
		return r.Update(context.TODO(), existingRoleBinding)
	}

	return nil
}

func getRoleRefFromRoleBinding(roleBinding client.Object) rbacv1.RoleRef {
	switch rb := roleBinding.(type) {
	case *rbacv1.RoleBinding:
		return rb.RoleRef
	case *rbacv1.ClusterRoleBinding:
		return rb.RoleRef
	}
	return rbacv1.RoleRef{}
}

func getSubjectsFromRoleBinding(roleBinding client.Object) []rbacv1.Subject {
	switch rb := roleBinding.(type) {
	case *rbacv1.RoleBinding:
		return rb.Subjects
	case *rbacv1.ClusterRoleBinding:
		return rb.Subjects
	}
	return nil
}

func setSubjectsOnRoleBinding(roleBinding client.Object, subjects []rbacv1.Subject) {
	switch rb := roleBinding.(type) {
	case *rbacv1.RoleBinding:
		rb.Subjects = subjects
	case *rbacv1.ClusterRoleBinding:
		rb.Subjects = subjects
	}
}

func (r *ReconcileArgoCD) reconcileSecretConfigMapHelper(cr *argoproj.ArgoCD, desiredResource client.Object) error {
	resourceExists := true
	resourceType := reflect.TypeOf(desiredResource).Elem().Name()
	existingResource := reflect.New(reflect.TypeOf(desiredResource).Elem()).Interface().(client.Object)
	if err := argoutil.FetchObject(r.Client, cr.Namespace, desiredResource.GetName(), existingResource); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the %s associated with %s : %s", resourceType, desiredResource.GetName(), err)
		}
		resourceExists = false
	}

	if resourceExists {
		// resource exists but shouldn't, so it should be deleted
		if !cr.Spec.ImageUpdater.Enabled {
			argoutil.LogResourceDeletion(log, existingResource, "image updater is disabled")
			return r.Delete(context.TODO(), existingResource)
		}

		// resource exists and should, nothing to do here
		return nil
	}

	// resource doesn't exist and shouldn't, nothing to do here
	if !cr.Spec.ImageUpdater.Enabled {
		return nil
	}

	// resource doesn't exist but should, so it should be created
	if err := controllerutil.SetControllerReference(cr, desiredResource, r.Scheme); err != nil {
		return err
	}

	argoutil.LogResourceCreation(log, desiredResource)
	err := r.Create(context.TODO(), desiredResource)
	if err != nil {
		return err
	}

	return nil
}

// getImageUpdaterResources will return the ResourceRequirements for the ImageUpdater container.
func getImageUpdaterResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.ImageUpdater.Resources != nil {
		resources = *cr.Spec.ImageUpdater.Resources
	}

	return resources
}
