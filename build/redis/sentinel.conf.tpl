dir "/data"
    sentinel down-after-milliseconds argocd 10000
    sentinel failover-timeout argocd 180000
    maxclients 10000
    sentinel parallel-syncs argocd 5
