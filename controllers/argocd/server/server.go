package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/openshift"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type ServerReconciler struct {
	Client                 client.Client
	Scheme                 *runtime.Scheme
	Instance               *argoproj.ArgoCD
	Logger                 *util.Logger
	ClusterScoped          bool
	ManagedNamespaces      map[string]string
	SourceNamespaces       map[string]string
	AppsetSourceNamespaces map[string]string

	RepoServer RepoServerController
	Redis      RedisController
	SSO        SSOController
}

var (
	resourceName               string
	managedNsResourceName      string
	sourceNsResourceName       string
	appsetSourceNsResourceName string
	grpcResourceName           string
	metricsResourceName        string
	clusterResourceName        string
	component                  string
)

func (sr *ServerReconciler) Reconcile() error {
	sr.varSetter()

	// perform resource reconciliation
	if err := sr.reconcileServiceAccount(); err != nil {
		sr.Logger.Error(err, "failed to reconcile service account")
		return err
	}

	if sr.ClusterScoped {
		if err := sr.reconcileClusterRole(); err != nil {
			sr.Logger.Error(err, "failed to reconcile clusterrole")
			return err
		}

		if err := sr.reconcileClusterRoleBinding(); err != nil {
			sr.Logger.Error(err, "failed to reconcile clusterrolebinding")
			return err
		}
	} else {
		if err := sr.deleteClusterRoleBinding(clusterResourceName); err != nil {
			sr.Logger.Error(err, "failed to delete clusterrolebinding")
		}

		if err := sr.deleteClusterRole(clusterResourceName); err != nil {
			sr.Logger.Error(err, "failed to delete clusterrole")
		}
	}

	if err := sr.reconcileRoles(); err != nil {
		sr.Logger.Error(err, "failed to reconcile one or more roles")
	}

	if err := sr.reconcileRoleBindings(); err != nil {
		sr.Logger.Error(err, "failed to reconcile one or more rolebindings")
	}

	if err := sr.reconcileDeployment(); err != nil {
		sr.Logger.Error(err, "failed to reconcile deployment")
		return err
	}

	if err := sr.reconcileServices(); err != nil {
		sr.Logger.Error(err, "failed to reconcile one or more services")
	}

	if monitoring.IsPrometheusAPIAvailable() {
		if sr.Instance.Spec.Prometheus.Enabled {
			if err := sr.reconcileMetricsServiceMonitor(); err != nil {
				sr.Logger.Error(err, "failed to reconcile metrics service monitor")
			}
		} else {
			if err := sr.deleteServiceMonitor(metricsResourceName, sr.Instance.Namespace); err != nil {
				sr.Logger.Error(err, "failed to delete serviceMonitor")
			}
		}
	} else {
		sr.Logger.Debug("prometheus API unavailable, skipping service monitor reconciliation")
	}

	if sr.Instance.Spec.Server.Autoscale.Enabled {
		if err := sr.reconcileHorizontalPodAutoscaler(); err != nil {
			sr.Logger.Error(err, "failed to reconcile HPA")
		}
	} else {
		if err := sr.deleteHorizontalPodAutoscaler(resourceName, sr.Instance.Namespace); err != nil {
			sr.Logger.Error(err, "failed to delete HPA")
		}
	}

	if err := sr.reconcileIngresses(); err != nil {
		sr.Logger.Error(err, "failed to reconcile one or more ingresses")
	}

	if openshift.IsOpenShiftEnv() {
		if sr.Instance.Spec.Server.Route.Enabled {
			if err := sr.reconcileRoute(); err != nil {
				sr.Logger.Error(err, "failed to reconcile route")
			}
		} else {
			// route disabled, cleanup any existing route and exit
			if err := sr.deleteRoute(resourceName, sr.Instance.Namespace); err != nil {
				sr.Logger.Error(err, "failed to delete route")
			}
		}
	}

	return nil
}

func (sr *ServerReconciler) DeleteResources() error {
	var deletionErr util.MultiError

	if openshift.IsOpenShiftEnv() {
		if err := sr.deleteRoute(resourceName, sr.Instance.Namespace); err != nil {
			sr.Logger.Error(err, "failed to delete route")
			deletionErr.Append(err)
		}
	}

	if err := sr.deleteIngresses(); err != nil {
		sr.Logger.Error(err, "failed to delete one or more ingresses")
		deletionErr.Append(err)
	}

	if err := sr.deleteHorizontalPodAutoscaler(resourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete HPA")
		deletionErr.Append(err)
	}

	if err := sr.deleteServices(); err != nil {
		sr.Logger.Error(err, "failed to delete one or more services")
		deletionErr.Append(err)
	}

	if err := sr.deleteServiceMonitor(metricsResourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete serviceMonitor")
		deletionErr.Append(err)
	}

	if err := sr.deleteDeployment(resourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := sr.deleteClusterRoleBinding(clusterResourceName); err != nil {
		sr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := sr.deleteClusterRole(clusterResourceName); err != nil {
		sr.Logger.Error(err, "failed to delete deployment")
		deletionErr.Append(err)
	}

	if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete control plane rolebinding")
		deletionErr.Append(err)
	}

	if err := sr.deleteRole(resourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete control plane role")
		deletionErr.Append(err)
	}

	// delete managed ns rbac
	roles, rbs, err := sr.getManagedNsRBAC()
	if err != nil {
		sr.Logger.Error(err, "failed to list one or more resource management namespace rbac resources")
	} else {
		if err := sr.DeleteRoleBindings(rbs); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane rolebindings")
			deletionErr.Append(err)
		}

		if err := sr.DeleteRoles(roles); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane roles")
			deletionErr.Append(err)
		}
	}

	// delete source ns rbac
	roles, rbs, err = sr.getSourceNsRBAC()
	if err != nil {
		sr.Logger.Error(err, "failed to list one or more app management namespace rbac resources")
	} else {
		if err := sr.DeleteRoleBindings(rbs); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane rolebindings")
			deletionErr.Append(err)
		}

		if err := sr.DeleteRoles(roles); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane roles")
			deletionErr.Append(err)
		}
	}

	// delete appset source ns rbac
	roles, rbs, err = sr.getAppsetSourceNsRBAC()
	if err != nil {
		sr.Logger.Error(err, "failed to list one or more appset management namespace rbac resources")
	} else {
		if err := sr.DeleteRoleBindings(rbs); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane rolebindings")
			deletionErr.Append(err)
		}

		if err := sr.DeleteRoles(roles); err != nil {
			sr.Logger.Error(err, "failed to delete one or more non control plane roles")
			deletionErr.Append(err)
		}
	}

	if err := sr.deleteServiceAccount(resourceName, sr.Instance.Namespace); err != nil {
		sr.Logger.Error(err, "failed to delete serviceaccount")
		deletionErr.Append(err)
	}

	return deletionErr.ErrOrNil()
}

func (sr *ServerReconciler) varSetter() {
	component = common.ServerComponent
	resourceName = argoutil.GenerateResourceName(sr.Instance.Name, common.ServerSuffix)
	clusterResourceName = argoutil.GenerateUniqueResourceName(sr.Instance.Name, sr.Instance.Namespace, common.ServerSuffix)

	grpcResourceName = argoutil.NameWithSuffix(resourceName, common.GRPCSuffix)
	metricsResourceName = argoutil.NameWithSuffix(resourceName, common.MetricsSuffix)
	managedNsResourceName = argoutil.NameWithSuffix(clusterResourceName, common.ResourceMgmtSuffix)
	sourceNsResourceName = argoutil.NameWithSuffix(clusterResourceName, common.AppMgmtSuffix)
	appsetSourceNsResourceName = argoutil.NameWithSuffix(clusterResourceName, common.AppsetMgmtSuffix)
}

func (sr *ServerReconciler) TriggerRollout(key string) error {
	return sr.TriggerDeploymentRollout(resourceName, sr.Instance.Namespace, key)
}

// deleteIngresses will delete all ArgoCD Server Ingress resources
func (sr *ServerReconciler) deleteIngresses() error {
	var reconErrs util.MultiError

	// delete server ingress
	if err := sr.deleteIngress(resourceName, sr.Instance.Namespace); err != nil {
		reconErrs.Append(err)
	}

	// delete server grpc ingress
	if err := sr.deleteIngress(grpcResourceName, sr.Instance.Namespace); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
}

// deleteServices will delete all ArgoCD Server service resources
func (sr *ServerReconciler) deleteServices() error {
	var reconErrs util.MultiError

	// delete server ingress
	if err := sr.deleteService(resourceName, sr.Instance.Namespace); err != nil {
		reconErrs.Append(err)
	}

	// delete server grpc ingress
	if err := sr.deleteService(metricsResourceName, sr.Instance.Namespace); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
}
