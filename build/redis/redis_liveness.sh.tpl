response=$(
  redis-cli \
    -h localhost \
    -p 6379 \
    ping
)
if [ "$response" != "PONG" ] && [ "${response:0:7}" != "LOADING" ] ; then
  echo "$response"
  exit 1
fi
echo "response=$response"
