package resource

import (
	"context"

	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateObject creates the specified object using the provided client.
func CreateObject(obj cntrlClient.Object, client cntrlClient.Client) error {
	return client.Create(context.TODO(), obj)
}

// GetObject retrieves the object with the given name and namespace using the provided client.
func GetObject(name, namespace string, obj cntrlClient.Object, client cntrlClient.Client) (cntrlClient.Object, error) {
	existingObj := obj.DeepCopyObject().(cntrlClient.Object)
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingObj)
	if err != nil {
		return nil, err
	}
	return existingObj, nil
}

// ListObjects returns a list of objects in the specified namespace using the provided client and list options.
func ListObjects(namespace string, objList cntrlClient.ObjectList, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (cntrlClient.ObjectList, error) {
	existingObjs := objList.DeepCopyObject().(cntrlClient.ObjectList)
	listOptions = append(listOptions, cntrlClient.InNamespace(namespace))
	err := client.List(context.TODO(), existingObjs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingObjs, nil
}

// UpdateObject updates the specified object using the provided client.
func UpdateObject(obj cntrlClient.Object, client cntrlClient.Client) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingObj, err := GetObject(obj.GetName(), obj.GetNamespace(), obj, client)
		if err != nil {
			return err
		}

		obj.SetResourceVersion(existingObj.GetResourceVersion())

		if err = client.Update(context.TODO(), obj); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		return err
	}
	return nil
}

// DeleteObject deletes the object with the given name and namespace using the provided client.
func DeleteObject(name, namespace string, obj cntrlClient.Object, client cntrlClient.Client) error {
	existingObj, err := GetObject(name, namespace, obj, client)
	if err != nil {
		return err
	}

	if err := client.Delete(context.TODO(), existingObj); err != nil {
		return err
	}
	return nil
}
