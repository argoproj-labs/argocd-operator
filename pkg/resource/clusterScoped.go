package resource

import (
	"context"

	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateClusterObject creates the specified object using the provided client.
func CreateClusterObject(obj cntrlClient.Object, client cntrlClient.Client) error {
	return client.Create(context.TODO(), obj)
}

// GetClusterObject retrieves the object with the given name using the provided client.
func GetClusterObject(name string, obj cntrlClient.Object, client cntrlClient.Client) (cntrlClient.Object, error) {
	existingObj := obj.DeepCopyObject().(cntrlClient.Object)
	err := client.Get(context.TODO(), types.NamespacedName{Name: name}, existingObj)
	if err != nil {
		return nil, err
	}
	return existingObj, nil
}

// ListClusterObjects returns a list of objects using the provided client and list options.
func ListClusterObjects(objList cntrlClient.ObjectList, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (cntrlClient.ObjectList, error) {
	existingObjs := objList.DeepCopyObject().(cntrlClient.ObjectList)
	err := client.List(context.TODO(), existingObjs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingObjs, nil
}

// UpdateClusterObject updates the specified object using the provided client.
func UpdateClusterObject(obj cntrlClient.Object, client cntrlClient.Client) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existingObj, err := GetClusterObject(obj.GetName(), obj, client)
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

// DeleteClusterObject deletes the object with the given name using the provided client.
func DeleteClusterObject(name string, obj cntrlClient.Object, client cntrlClient.Client) error {
	existingObj, err := GetClusterObject(name, obj, client)
	if err != nil {
		return err
	}

	if err := client.Delete(context.TODO(), existingObj); err != nil {
		return err
	}
	return nil
}
