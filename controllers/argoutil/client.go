package argoutil

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client" // Renamed to avoid conflict with our wrapper's name

	"github.com/argoproj-labs/argocd-operator/common"
)

// ClientWrapper wraps a controller-runtime client to customize Get behavior.
// It embeds the cachedClient, allowing other methods (Create, Update, Delete etc.)
// to be directly inherited from the cachedClient's implementation.
// For Get, it falls back to liveClient on cache miss and labels the object.
type ClientWrapper struct {
	ctrlclient.Client                   // Embedded cached client. This provides all methods by default.
	liveClient        ctrlclient.Client // Client for direct API server calls on cache misses.
}

// NewClientWrapper creates a new ClientWrapper.
// `cachedClient` is typically the client from the Manager (which uses the cache and respects cache filters).
// `liveClient` is a client configured to bypass the cache and talk directly to the API server.
func NewClientWrapper(cachedClient ctrlclient.Client, liveClient ctrlclient.Client) *ClientWrapper {
	return &ClientWrapper{
		Client:     cachedClient, // Embed the cached client
		liveClient: liveClient,
	}
}

// Get overrides the embedded client's Get method to implement cache fallback.
func (cw *ClientWrapper) Get(ctx context.Context, key types.NamespacedName, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	// Try getting from the cached client first (this is the embedded client's Get)
	err := cw.Client.Get(ctx, key, obj, opts...)
	if err == nil {
		// Found in cache, return successfully
		return nil
	}
	// If not found in cache, check if it's a NotFound error
	if errors.IsNotFound(err) {
		// Use liveClient to do a live lookup of resource
		liveErr := cw.liveClient.Get(ctx, key, obj, opts...)
		if liveErr == nil {
			// Resource found live - try to label it for future caching
			// Create patch from the original live state
			patch := ctrlclient.MergeFrom(obj.DeepCopyObject().(ctrlclient.Object))
			// Add the tracking label to match cache filtering
			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[common.ArgoCDTrackedByOperatorLabel] = common.ArgoCDAppName
			obj.SetLabels(labels)
			// Attempt to patch the live object with the new label
			// If patching fails, we still succeeded in getting the resource
			// Return success - we found the object live
			_ = cw.liveClient.Patch(ctx, obj, patch)
			return nil
		}
		// Object not found in both cache and live - return the live error
		return liveErr
	}

	// For other errors (e.g., API server down, permissions), return the original cache error
	return err
}
