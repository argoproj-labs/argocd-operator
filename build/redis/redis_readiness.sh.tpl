response=$(
  redis-cli \
    -a "${AUTH}" --no-auth-warning \
    -h localhost \
    -p 6379 \
{{- if eq .UseTLS "true"}}
    --tls \
    --cacert /app/config/redis/tls/tls.crt \
{{- end}}
    ping
)
if [ "$response" != "PONG" ] ; then
  echo "$response"
  exit 1
fi
echo "response=$response"
