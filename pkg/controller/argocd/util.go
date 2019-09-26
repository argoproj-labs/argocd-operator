package argocd

import (
	"context"

	configv1 "github.com/openshift/api/config/v1"
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

func isOpenshift() bool {
	return isOpenshiftCluster
}

// VerifyOpenshift will verift that the OpenShift API is present, indicating an OpenShift cluster.
func VerifyOpenshift() error {
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
		Group:   configv1.GroupName,
		Version: configv1.GroupVersion.Version,
	}

	err = discovery.ServerSupportsVersion(k8s, gv)
	isOpenshiftCluster = (err == nil)
	return nil
}
