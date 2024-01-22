package redis

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	cntrlr "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RedisReconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Instance *argoproj.ArgoCD
	Logger   logr.Logger

	Appcontroller AppController
	Server        ServerController
	RepoServer    RepoServerController
	TLSEnabled    bool
}

var (
	resourceName   string
	component      string
	resourceLabels map[string]string
)

func (rr *RedisReconciler) Reconcile() error {
	rr.Logger = cntrlr.Log.WithName(common.RedisController).WithValues("instance", rr.Instance.Name, "instance-namespace", rr.Instance.Namespace)
	component = common.RedisComponent
	resourceName = argoutil.GenerateResourceName(rr.Instance.Name, component)
	resourceLabels = common.DefaultResourceLabels(resourceName, rr.Instance.Name, component)

	// check if TLS needs to be used
	rr.TLSEnabled = rr.UseTLS()

	if rr.Instance.Spec.HA.Enabled {
		// clean up regular redis resources first
		if err := rr.DeleteNonHAResources(); err != nil {
			rr.Logger.Error(err, "failed to delete non HA redis resources")
		}

		// reconcile HA resources
		if err := rr.reconcileHA(); err != nil {
			rr.Logger.Error(err, "failed to reconcile resources in HA mode")
			return err
		}
	} else {
		// clean up redis HA resources
		if err := rr.DeleteHAResources(); err != nil {
			rr.Logger.Error(err, "failed to delete redis HA resources")
		}

		// reconcile redis resources
		if err := rr.reconcile(); err != nil {
			rr.Logger.Error(err, "failed to reconcile resources")
			return err
		}
	}

	return nil
}

func (rr *RedisReconciler) reconcile() error {
	// reconcile role
	if err := rr.reconcileRole(); err != nil {
		return err
	}

	// reconcile serviceaccount
	if err := rr.reconcileServiceAccount(); err != nil {
		return err
	}

	// reconcile rolebinding
	if err := rr.reconcileRoleBinding(); err != nil {
		return err
	}

	// reconcile service
	if err := rr.reconcileService(); err != nil {
		return err
	}

	// reconcile Deployment
	if err := rr.reconcileDeployment(); err != nil {
		return err
	}

	// reconcile TLS secret
	if err := rr.reconcileTLSSecret(); err != nil {
		return err
	}
	return nil
}

func (rr *RedisReconciler) TriggerRollout(key string) error {
	if rr.Instance.Spec.HA.Enabled {
		if err := rr.TriggerHARollout(key); err != nil {
			rr.Logger.Error(err, "TriggerRollout: failed to rollout redis resources")
			return err
		}
		return nil
	} else {
		if err := rr.TriggerDeploymentRollout(resourceName, rr.Instance.Namespace, key); err != nil {
			rr.Logger.Error(err, "TriggerRollout: failed to rollout redis deployment")
			return err
		}
	}
	return nil
}

// delete overlapping resources with HA when switching to HA
func (rr *RedisReconciler) DeleteNonHAResources() error {
	var deletionErr util.MultiError

	// delete deployment
	err := rr.deleteDeployment(resourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete service
	err = rr.deleteService(resourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete role
	err = rr.deleteRole(resourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	return deletionErr.ErrOrNil()
}

// delete all redis resources
func (rr *RedisReconciler) DeleteResources() error {
	var deletionErr util.MultiError

	err := rr.DeleteHAResources()
	deletionErr.Append(err)

	err = rr.DeleteNonHAResources()
	deletionErr.Append(err)

	// delete rolebinding
	err = rr.deleteRoleBinding(resourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete serviceaccount
	err = rr.deleteServiceAccount(resourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete TLS secret
	err = rr.deleteSecret(common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace)
	deletionErr.Append(err)

	return deletionErr.ErrOrNil()
}
