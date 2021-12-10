response=$(
  redis-cli \
    -h localhost \
    -p 26379 \
    ping
)
if [ "$response" != "PONG" ]; then
  echo "$response"
  exit 1
fi
echo "response=$response"
