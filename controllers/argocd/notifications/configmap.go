package notifications

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileConfigMap() error {

	nr.Logger.Info("reconciling configMaps")

	name := argoutil.GenerateUniqueResourceName(nr.Instance.Name, nr.Instance.Namespace, ArgoCDNotificationsControllerComponent)
	labels := common.DefaultLabels(name, nr.Instance.Name, ArgoCDNotificationsControllerComponent)
	labels[common.AppK8sKeyName] = name

	configMapRequest := workloads.ConfigMapRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   nr.Instance.Namespace,
			Labels:      labels,
			Annotations: nr.Instance.Annotations,
		},
	}

	desiredConfigMap, err := workloads.RequestConfigMap(configMapRequest)

	if err != nil {
		nr.Logger.Error(err, "reconcileConfigMap: failed to request configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		nr.Logger.V(1).Info("reconcileConfigMap: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileConfigMap: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.DeleteConfigMap(desiredConfigMap.Name, desiredConfigMap.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to delete configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		}
		return err
	}

	existingConfigMap, err := workloads.GetConfigMap(desiredConfigMap.Name, desiredConfigMap.Namespace, nr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileConfigMap: failed to retrieve configMap", "name", existingConfigMap.Name, "namespace", existingConfigMap.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredConfigMap, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to set owner reference for configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredConfigMap, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileConfigMap: failed to create configMap", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
			return err
		}
		nr.Logger.V(0).Info("reconcileConfigMap: configMap created", "name", desiredConfigMap.Name, "namespace", desiredConfigMap.Namespace)
		return nil
	}

	nr.Logger.V(0).Info("reconcileConfigMap: configMap updated", "name", existingConfigMap.Name, "namespace", existingConfigMap.Namespace)
	return nil
}

func (nr *NotificationsReconciler) DeleteConfigMap(name, namespace string) error {
	if err := workloads.DeleteConfigMap(name, namespace, nr.Client); err != nil {
		nr.Logger.Error(err, "DeleteConfigMap: failed to delete configMap", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.V(0).Info("DeleteConfigMap: configMap deleted", "name", name, "namespace", namespace)
	return nil
}
