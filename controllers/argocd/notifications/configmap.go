package notifications

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileConfigMap() error {

	nr.Logger.Info("reconciling configMaps")

	configMapRequest := workloads.ConfigMapRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.NotificationsConfigMapName,
			Namespace:   nr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: nr.Instance.Annotations,
		},
		Data: GetDefaultNotificationsConfig(),
	}

	desiredConfigMap, err := workloads.RequestConfigMap(configMapRequest)

	if err != nil {
		nr.Logger.Error(err, "reconcileConfigMap: failed to request configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		nr.Logger.Debug("reconcileConfigMap: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileConfigMap: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.deleteConfigMap(desiredConfigMap.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to delete configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		}
		return err
	}

	_, err = workloads.GetConfigMap(desiredConfigMap.Name, desiredConfigMap.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileConfigMap: failed to retrieve configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredConfigMap, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to set owner reference for configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredConfigMap, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to create configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
			return err
		}
		nr.Logger.Info("configMap created", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		return nil
	}

	return nil
}

func (nr *NotificationsReconciler) deleteConfigMap(namespace string) error {
	if err := workloads.DeleteConfigMap(common.NotificationsConfigMapName, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteConfigMap: failed to delete configMap", "name", common.NotificationsConfigMapName, "namespace", namespace)
		return err
	}
	nr.Logger.Info("configMap deleted", "name", common.NotificationsConfigMapName, "namespace", namespace)
	return nil
}
