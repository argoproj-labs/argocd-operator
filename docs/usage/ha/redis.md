# High Availability for Redis

The Argo CD operator supports high availability through the mechanism described in the Argo CD [documentation][argocd_ha].

To enable HA for an Argo CD cluster, include the `ha` section in the `ArgoCD` Custom Resource.

!!! note
    When `ha` is enabled, changes to `.spec.redis.resources` doesn't have any effect. Redis resource limits can be set using `.spec.ha.resources`.

``` yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: example-argocd
  labels:
    example: ha
spec:
  ha:
    enabled: true
    redisProxyImage: haproxy
    redisProxyVersion: "2.0.4"
```

## OpenShift

When running the Argo CD operator on OpenShift, you must apply the `anyuid` SCC to the Service Account for Redis prior to creating an `ArgoCD` Custom Resource.

``` bash
oc adm policy add-scc-to-user anyuid -z argocd-redis-ha
```

If the above step is not performed, the StatefulSet for HA Redis will not be able to create Pods.

[argocd_ha]:https://argoproj.github.io/argo-cd/operator-manual/high_availability
