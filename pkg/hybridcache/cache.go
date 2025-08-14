package hybridcache

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/argoproj-labs/argocd-operator/common"
)

// LabelFilteredCache provides a controller-runtime cache and client that only
// cache/read corev1 Secrets and ConfigMaps bearing a specific label selector.
// Reads go through the filtered cache; writes and status updates go to the API
// server directly via the underlying client.
type LabelFilteredCache struct {
	Cache  crcache.Cache
	Client crclient.Client
}

// NewLabelFilteredCache returns a cache+client pair that only stores
// corev1.Secrets and corev1.ConfigMaps labeled with
// operator.argoproj.io/tracked-by=argocd. The returned client is configured
// to use this cache as its Reader.
func NewLabelFilteredCache(cfg *rest.Config) (*LabelFilteredCache, error) {
	fc := &LabelFilteredCache{}
	selector := labels.SelectorFromSet(labels.Set{
		common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
	})

	schema := runtime.NewScheme()
	if err := corev1.AddToScheme(schema); err != nil {
		return nil, fmt.Errorf("add corev1 to scheme: %w", err)
	}

	// Create a cache that only includes Secrets and ConfigMaps with the specified label
	c, err := crcache.New(cfg, crcache.Options{
		Scheme: schema,
		ByObject: map[crclient.Object]crcache.ByObject{
			&corev1.Secret{}:    {Label: selector},
			&corev1.ConfigMap{}: {Label: selector},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create filtered cache: %w", err)
	}
	fc.Cache = c

	// Create a client that uses the filtered cache
	cl, err := crclient.New(cfg, crclient.Options{
		Scheme: schema,
		Cache:  &crclient.CacheOptions{Reader: fc.Cache},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client for filtered cache: %w", err)
	}
	fc.Client = cl

	return fc, nil
}

// AttachToManager starts the cache with the manager and blocks manager start
// until caches have synced. Any start error from the cache is returned to the mgr.
func (fc *LabelFilteredCache) AttachToManager(mgr manager.Manager) error {
	return mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		errCh := make(chan error, 1)

		// Start the cache
		go func() {
			if err := fc.Cache.Start(ctx); err != nil {
				errCh <- fmt.Errorf("filtered cache start failed: %w", err)
				return
			}
			close(errCh)
		}()

		// Wait for initial sync or an early start error
		if ok := fc.Cache.WaitForCacheSync(ctx); !ok {
			return fmt.Errorf("filtered cache failed to sync")
		}

		// If Start errored early, surface it
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		default:
		}

		// Keep running until manager stops
		<-ctx.Done()
		return nil
	}))
}
