package argocd

import (
	"context"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

var isOpenshiftCluster = false

func (r *ReconcileArgoCD) isObjectFound(nsname types.NamespacedName, obj runtime.Object) bool {
	err := r.client.Get(context.TODO(), nsname, obj)
	if err != nil {
		return false
	}
	return true
}

// IsOpenShift returns true if the operator is running in an OpenShift environment.
func IsOpenShift() bool {
	return isOpenshiftCluster
}

// VerifyOpenShift will verify that the OpenShift API is present, indicating an OpenShift cluster.
func VerifyOpenShift() error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return err
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return err
	}

	gv := schema.GroupVersion{
		Group:   routev1.GroupName,
		Version: routev1.GroupVersion.Version,
	}

	err = discovery.ServerSupportsVersion(k8s, gv)
	if err == nil {
		log.Info("openshift verified")
		isOpenshiftCluster = true
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileOpenShiftResources(cr *argoproj.ArgoCD) error {
	if err := r.reconcileRoutes(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheus(cr); err != nil {
		return err
	}

	if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
		return err
	}

	if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
		return err
	}

	if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileResources(cr *argoproj.ArgoCD) error {
	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}
	return nil
}
