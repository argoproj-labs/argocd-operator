package argoutil

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/common"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client" // Renamed to avoid conflict with our wrapper's name
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
		// Found in cache, return
		return nil
	}
	// If not found in cache, check if it's a NotFound error
	if errors.IsNotFound(err) {
		log.Info("Resource not found in cache, attempting live lookup",
			"kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name)

		// Use liveClient to do a live look up of resource
		liveErr := cw.liveClient.Get(ctx, key, obj, opts...)

		if liveErr == nil {
			log.Info("Resource found live, attempting to label for future caching",
				"kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name)

			// Resource is present live, add the label so it ends up in the cache next time
			patch := ctrlclient.MergeFrom(obj.DeepCopyObject().(ctrlclient.Object)) // Patch from original live state

			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			labels[common.ArgoCDKeyPartOf] = common.ArgoCDAppName
			obj.SetLabels(labels)

			// Attempt to patch the live object with the new label
			patchErr := cw.liveClient.Patch(ctx, obj, patch)
			if patchErr != nil {
				// Log the error but don't fail the Get operation if we successfully retrieved it live
				log.Error(patchErr, "Failed to label live-looked-up resource",
					"kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name)
			} else {
				log.Info("Successfully labeled resource",
					"kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name)
			}
			return nil
		}
		log.Info("Resource not found live",
			"kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name, "error", liveErr.Error())
		return err // Return the original cache "not found" error
	}
	// For other errors (e.g., API server down, permissions), return the original error from the cache lookup
	log.Error(err, "Error getting resource from cache", "kind", obj.GetObjectKind().GroupVersionKind().Kind, "namespace", key.Namespace, "name", key.Name)
	return err
}
