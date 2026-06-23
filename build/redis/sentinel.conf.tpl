dir "/data"
{{- if eq .UseTLS "false"}}
port 26379
{{- else}}
port 0
tls-port 26379
tls-cert-file /app/config/redis/tls/tls.crt
tls-ca-cert-file /app/config/redis/tls/tls.crt
tls-key-file /app/config/redis/tls/tls.key
tls-replication yes
tls-auth-clients no
{{- end}}
bind 0.0.0.0
    sentinel down-after-milliseconds argocd 10000
    sentinel failover-timeout argocd 180000
    maxclients 10000
    sentinel parallel-syncs argocd 5
    sentinel auth-pass argocd __REPLACE_DEFAULT_AUTH__
