package applicationset

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type ApplicationSetReconciler struct {
	Client                 client.Client
	Scheme                 *runtime.Scheme
	Instance               *argoproj.ArgoCD
	Logger                 *util.Logger
	ClusterScoped          bool
	AppsetSourceNamespaces map[string]string

	RepoServer RepoServerController
}

var (
	resourceName               string
	clusterResourceName        string
	component                  string
	appsetSourceNsResourceName string
	webhookResourceName        string
)

func (asr *ApplicationSetReconciler) Reconcile() error {
	asr.varSetter()

	if err := asr.reconcileServiceAccount(); err != nil {
		asr.Logger.Info("reconciling applicationSet serviceaccount")
		return err
	}

	if asr.ClusterScoped {
		if err := asr.reconcileClusterRole(); err != nil {
			asr.Logger.Error(err, "failed to reconcile clusterrole")
			return err
		}

		if err := asr.reconcileClusterRoleBinding(); err != nil {
			asr.Logger.Error(err, "failed to reconcile clusterrolebinding")
			return err
		}
	} else {
		if err := asr.deleteClusterRoleBinding(clusterResourceName); err != nil {
			asr.Logger.Error(err, "failed to delete clusterrolebinding")
		}

		if err := asr.deleteClusterRole(clusterResourceName); err != nil {
			asr.Logger.Error(err, "failed to delete clusterrole")
		}
	}

	if err := asr.reconcileRoles(); err != nil {
		asr.Logger.Error(err, "failed to reconcile one or more roles")
	}

	if err := asr.reconcileRolebindings(); err != nil {
		asr.Logger.Error(err, "failed to reconcile one or more rolebindings")
	}

	if openshift.IsOpenShiftEnv() {
		if asr.Instance.Spec.ApplicationSet.WebhookServer.Route.Enabled {
			if err := asr.reconcileWebhookRoute(); err != nil {
				asr.Logger.Error(err, "failed to reconcile webhook route")
				return err
			}
		} else {
			if err := asr.deleteRoute(webhookResourceName, asr.Instance.Namespace); err != nil {
				asr.Logger.Error(err, "failed to delete webhook route")
			}
		}
	}

	if asr.Instance.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
		if err := asr.reconcileIngress(); err != nil {
			asr.Logger.Error(err, "failed to reconcile ingress")
			return err
		}
	} else {
		if err := asr.deleteIngress(resourceName, asr.Instance.Namespace); err != nil {
			asr.Logger.Error(err, "failed to delete ingress")
		}
	}

	if err := asr.reconcileService(); err != nil {
		asr.Logger.Error(err, "failed to reconcile service")
		return err
	}

	if err := asr.reconcileDeployment(); err != nil {
		asr.Logger.Error(err, "failed to reconcile deployment")
		return err
	}

	return nil
}

func (asr *ApplicationSetReconciler) DeleteResources() error {

	var deletionErr util.MultiError

	if err := asr.deleteDeployment(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := asr.deleteService(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "failed to delete service")
		deletionErr.Append(err)
	}

	if err := asr.deleteIngress(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "failed to delete ingress")
		deletionErr.Append(err)
	}

	if openshift.IsOpenShiftEnv() {
		if err := asr.deleteRoute(webhookResourceName, asr.Instance.Namespace); err != nil {
			asr.Logger.Error(err, "failed to delete webhook route")
			deletionErr.Append(err)
		}
	}

	if err := asr.deleteRoleBinding(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete roleBinding")
		deletionErr.Append(err)
	}

	if err := asr.deleteRole(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete role")
		deletionErr.Append(err)
	}

	// delete appset source ns rbac
	roles, rbs, err := asr.getAppsetSourceNsRBAC()
	if err != nil {
		asr.Logger.Error(err, "failed to list one or more appset management namespace rbac resources")
	} else {
		if err := asr.DeleteRoleBindings(rbs); err != nil {
			asr.Logger.Error(err, "failed to delete one or more non control plane rolebindings")
			deletionErr.Append(err)
		}

		if err := asr.DeleteRoles(roles); err != nil {
			asr.Logger.Error(err, "failed to delete one or more non control plane roles")
			deletionErr.Append(err)
		}
	}

	if err := asr.deleteClusterRoleBinding(clusterResourceName); err != nil {
		asr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := asr.deleteClusterRole(clusterResourceName); err != nil {
		asr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := asr.deleteServiceAccount(resourceName, asr.Instance.Namespace); err != nil {
		asr.Logger.Error(err, "DeleteResources: failed to delete serviceaccount")
		deletionErr.Append(err)
	}

	return deletionErr.ErrOrNil()
}

func (asr *ApplicationSetReconciler) varSetter() {
	component = common.AppSetControllerComponent
	resourceName = argoutil.GenerateResourceName(asr.Instance.Name, common.AppSetControllerSuffix)
	clusterResourceName = argoutil.GenerateUniqueResourceName(asr.Instance.Name, asr.Instance.Namespace, common.AppSetControllerSuffix)

	webhookResourceName = argoutil.NameWithSuffix(resourceName, common.WebhookSuffix)
	appsetSourceNsResourceName = argoutil.NameWithSuffix(clusterResourceName, common.AppsetMgmtSuffix)
}
