package hybridcache

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// HybridClient wraps controller-runtime clients to customize Get/List behavior.
// It embeds a primary cache-backed client (typically the manager's client)
// so methods like Create/Update/Delete are inherited. Reads for certain core
// types (Secrets, ConfigMaps) are routed through a metadata probe + filtered
// cache, with a live fallback that also applies a tracking label.
type HybridClient struct {
	crclient.Client                 // primary cache-backed client (often the mgr client)
	labelClient     crclient.Client // client backed with label-filtered Secrets/ConfigMaps cache
	liveClient      crclient.Client // client for live lookups (bypassing cache)
}

// NewHybridClient creates a new HybridClient.
func NewHybridClient(primaryClient crclient.Client, labelClient crclient.Client, liveClient crclient.Client) *HybridClient {
	return &HybridClient{
		Client:      primaryClient,
		labelClient: labelClient,
		liveClient:  liveClient,
	}
}

// Get overrides the embedded client's Get to implement a three-step flow for
// Secrets/ConfigMaps:
//  1. Probe via PartialObjectMetadata against the primary (metadata) cache.
//     If the object doesn't exist, return the error immediately.
//  2. Attempt to read the full object from the label-filtered cache/client.
//     If found, verify ResourceVersion matches the metadata probe to avoid
//     serving a stale entry.
//  3. If not cached (or stale), fetch live from the API and ensure the
//     tracking label is present so the filtered cache can store it next time.
//
// For all other types, it defers to the primary client (full-object cache).
func (hc *HybridClient) Get(ctx context.Context, key types.NamespacedName, obj crclient.Object, opts ...crclient.GetOption) error {
	switch obj.(type) {
	case *corev1.Secret, *corev1.ConfigMap:
		// Step 1: POM probe
		pom := &metav1.PartialObjectMetadata{}
		g, v, k := gvkFor(obj)
		pom.SetGroupVersionKind(schema.GroupVersionKind{Group: g, Version: v, Kind: k})
		if err := hc.Client.Get(ctx, key, pom, opts...); err != nil {
			return err
		}

		// Step 2: try label-filtered full-object cache
		err := hc.labelClient.Get(ctx, key, obj, opts...)
		if err == nil {
			// safety check against stale cache entries
			if obj.GetResourceVersion() != pom.GetResourceVersion() {
				return fmt.Errorf("stale cache entry for %s/%s", key.Namespace, key.Name)
			}
			// Cache hit, no need to go live
			return nil
		} else if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		// Step 3: live get + label
		if err := hc.liveClient.Get(ctx, key, obj); err != nil {
			return err
		}
		// Ensure the object has the tracking label
		return hc.ensureTrackingLabel(ctx, obj)

	default:
		return hc.Client.Get(ctx, key, obj, opts...)
	}
}

// List overrides the embedded client's List for Secrets/ConfigMaps to return a
// **metadata-only** list built from PartialObjectMetadata items in the primary
// cache. Each item in the returned list contains ObjectMeta only; fields like
// `data`, `binaryData`, or `immutable` are not populated. Callers that need
// full objects should follow up with Get() per item.
//
// For other types, it defers to the primary client which returns full objects.
func (hc *HybridClient) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	switch out := list.(type) {
	case *corev1.SecretList:
		metaList := &metav1.PartialObjectMetadataList{}
		metaList.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "SecretList"})
		if err := hc.Client.List(ctx, metaList, opts...); err != nil {
			return err
		}
		out.Items = make([]corev1.Secret, len(metaList.Items))
		for i := range metaList.Items {
			// NOTE: Metadata-only list; data/spec omitted.
			// TODO: If full objects are needed, call Get() for each before returning.
			// For now, callers should Get() each item as needed.
			out.Items[i] = corev1.Secret{ObjectMeta: metaList.Items[i].ObjectMeta}
		}
		
		return nil

	case *corev1.ConfigMapList:
		metaList := &metav1.PartialObjectMetadataList{}
		metaList.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMapList"})
		if err := hc.Client.List(ctx, metaList, opts...); err != nil {
			return err
		}
		out.Items = make([]corev1.ConfigMap, len(metaList.Items))
		for i := range metaList.Items {
			// NOTE: Metadata-only list; data/spec omitted.
			// TODO: If full objects are needed, call Get() for each before returning.
			// For now, callers should Get() each item as needed.
			out.Items[i] = corev1.ConfigMap{ObjectMeta: metaList.Items[i].ObjectMeta}
		}
		return nil

	default:
		// For other object types, use the primary client directly
		// which will query the default cache where full objects are stored.
		return hc.Client.List(ctx, list, opts...)
	}
}

// ensureTrackingLabel ensures that the object has the tracking label set.
// This is used to ensure that the object can be cached in the future.
func (hc *HybridClient) ensureTrackingLabel(ctx context.Context, obj crclient.Object) error {
	orig := obj.DeepCopyObject().(crclient.Object)
	ls := obj.GetLabels()
	if ls == nil {
		ls = map[string]string{}
	}
	if curr, ok := ls[common.ArgoCDTrackedByOperatorLabel]; ok && curr == common.ArgoCDAppName {
		return nil
	}
	ls[common.ArgoCDTrackedByOperatorLabel] = common.ArgoCDAppName
	obj.SetLabels(ls)
	patch := crclient.MergeFrom(orig)

	// Attempt to patch the live object with the new label
	// If patching fails, we still succeeded in getting the resource
	_ = hc.Client.Patch(ctx, obj, patch)
	return nil
}

func gvkFor(obj crclient.Object) (string, string, string) {
	switch obj.(type) {
	case *corev1.Secret:
		return "", "v1", "Secret"
	case *corev1.ConfigMap:
		return "", "v1", "ConfigMap"
	default:
		return "", "", ""
	}
}
