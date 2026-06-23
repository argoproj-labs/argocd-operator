response=$(
  redis-cli \
    -h localhost \
    -p 26379 \
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
