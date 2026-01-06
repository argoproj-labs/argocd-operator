redis_pwd="$(cat /redis-initial-pass/admin.password)"
response=$(
  env REDISCLI_AUTH="${redis_pwd}" redis-cli \
    -h localhost \
    -p 6379 \
{{- if eq .UseTLS "true"}}
    --tls \
    --cacert /app/config/redis/tls/tls.crt \
{{- end}}
    ping
)
if [ "$response" != "PONG" ]; then
  echo "$response"
  exit 1
fi
echo "response=$response"
