{{- if eq .UseTLS "true"}}
global
    ca-base /app/config/redis/tls
{{- end}}

defaults REDIS
    mode tcp
    timeout connect 4s
    timeout server 6m
    timeout client 6m
    timeout check 2s

listen health_check_http_url
    bind :8888
    mode http
    monitor-uri /healthz
    option      dontlognull
# Check Sentinel and whether they are nominated master
backend check_if_redis_is_master_0
    mode tcp
    option tcp-check
{{- if eq .UseTLS "false"}}
    tcp-check connect
{{- else}}
    tcp-check connect ssl
{{- end}}
    tcp-check send "AUTH __REPLACE_DEFAULT_AUTH__"\r\n
    tcp-check expect string +OK
    tcp-check send PING\r\n
    tcp-check expect string +PONG
    tcp-check send SENTINEL\ get-master-addr-by-name\ argocd\r\n
    tcp-check expect string REPLACE_ANNOUNCE0
    tcp-check send QUIT\r\n
    tcp-check expect string +OK
{{- if eq .UseTLS "false"}}
    server R0 {{.ServiceName}}-announce-0:26379 check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 check inter 3s
{{- else}}
    server R0 {{.ServiceName}}-announce-0:26379 verify required ca-file tls.crt check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 verify required ca-file tls.crt check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 verify required ca-file tls.crt check inter 3s
{{- end}}
# Check Sentinel and whether they are nominated master
backend check_if_redis_is_master_1
    mode tcp
    option tcp-check
{{- if eq .UseTLS "false"}}
    tcp-check connect
{{- else}}
    tcp-check connect ssl
{{- end}}
    tcp-check send "AUTH __REPLACE_DEFAULT_AUTH__"\r\n
    tcp-check expect string +OK
    tcp-check send PING\r\n
    tcp-check expect string +PONG
    tcp-check send SENTINEL\ get-master-addr-by-name\ argocd\r\n
    tcp-check expect string REPLACE_ANNOUNCE1
    tcp-check send QUIT\r\n
    tcp-check expect string +OK
{{- if eq .UseTLS "false"}}
    server R0 {{.ServiceName}}-announce-0:26379 check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 check inter 3s
{{- else}}
    server R0 {{.ServiceName}}-announce-0:26379 verify required ca-file tls.crt check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 verify required ca-file tls.crt check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 verify required ca-file tls.crt check inter 3s
{{- end}}
# Check Sentinel and whether they are nominated master
backend check_if_redis_is_master_2
    mode tcp
    option tcp-check
{{- if eq .UseTLS "false"}}
    tcp-check connect
{{- else}}
    tcp-check connect ssl
{{- end}}
    tcp-check send "AUTH __REPLACE_DEFAULT_AUTH__"\r\n
    tcp-check expect string +OK
    tcp-check send PING\r\n
    tcp-check expect string +PONG
    tcp-check send SENTINEL\ get-master-addr-by-name\ argocd\r\n
    tcp-check expect string REPLACE_ANNOUNCE2
    tcp-check send QUIT\r\n
    tcp-check expect string +OK
{{- if eq .UseTLS "false"}}
    server R0 {{.ServiceName}}-announce-0:26379 check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 check inter 3s
{{- else}}
    server R0 {{.ServiceName}}-announce-0:26379 verify required ca-file tls.crt check inter 3s
    server R1 {{.ServiceName}}-announce-1:26379 verify required ca-file tls.crt check inter 3s
    server R2 {{.ServiceName}}-announce-2:26379 verify required ca-file tls.crt check inter 3s
{{- end}}

# decide redis backend to use
#master
frontend ft_redis_master
    bind *:6379
    use_backend bk_redis_master
# Check all redis servers to see if they think they are master
backend bk_redis_master
    mode tcp
    option tcp-check
{{- if eq .UseTLS "false"}}
    tcp-check connect
{{- else}}
    tcp-check connect ssl
{{- end}}
    tcp-check send "AUTH __REPLACE_DEFAULT_AUTH__"\r\n
    tcp-check expect string +OK
    tcp-check send PING\r\n
    tcp-check expect string +PONG
    tcp-check send info\ replication\r\n
    tcp-check expect string role:master
    tcp-check send QUIT\r\n
    tcp-check expect string +OK
{{- if eq .UseTLS "false"}}
    use-server R0 if { srv_is_up(R0) } { nbsrv(check_if_redis_is_master_0) ge 2 }
    server R0 {{.ServiceName}}-announce-0:6379 check inter 3s fall 1 rise 1
    use-server R1 if { srv_is_up(R1) } { nbsrv(check_if_redis_is_master_1) ge 2 }
    server R1 {{.ServiceName}}-announce-1:6379 check inter 3s fall 1 rise 1
    use-server R2 if { srv_is_up(R2) } { nbsrv(check_if_redis_is_master_2) ge 2 }
    server R2 {{.ServiceName}}-announce-2:6379 check inter 3s fall 1 rise 1
{{- else}}
    use-server R0 if { srv_is_up(R0) } { nbsrv(check_if_redis_is_master_0) ge 2 }
    server R0 {{.ServiceName}}-announce-0:6379 verify required ca-file tls.crt check inter 3s fall 1 rise 1
    use-server R1 if { srv_is_up(R1) } { nbsrv(check_if_redis_is_master_1) ge 2 }
    server R1 {{.ServiceName}}-announce-1:6379 verify required ca-file tls.crt check inter 3s fall 1 rise 1
    use-server R2 if { srv_is_up(R2) } { nbsrv(check_if_redis_is_master_2) ge 2 }
    server R2 {{.ServiceName}}-announce-2:6379 verify required ca-file tls.crt check inter 3s fall 1 rise 1
{{- end}}
