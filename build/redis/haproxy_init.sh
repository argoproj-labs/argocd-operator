: "${ARGOCD_REDIS_SERVICE_NAME:?Variable ARGOCD_REDIS_SERVICE_NAME is required}"

HAPROXY_CONF=/data/haproxy.cfg
cp /readonly/haproxy.cfg "$HAPROXY_CONF"

for loop in $(seq 1 10); do
    getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-0" && break
    echo "Waiting for service ${ARGOCD_REDIS_SERVICE_NAME}-announce-0 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP0=$(getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-0" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP0" ]; then
    echo "Could not resolve the announce ip for ${ARGOCD_REDIS_SERVICE_NAME}-announce-0"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE0/$ANNOUNCE_IP0/" "$HAPROXY_CONF"

for loop in $(seq 1 10); do
    getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-1" && break
    echo "Waiting for service ${ARGOCD_REDIS_SERVICE_NAME}-announce-1 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP1=$(getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-1" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP1" ]; then
    echo "Could not resolve the announce ip for ${ARGOCD_REDIS_SERVICE_NAME}-announce-1"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE1/$ANNOUNCE_IP1/" "$HAPROXY_CONF"

for loop in $(seq 1 10); do
    getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-2" && break
    echo "Waiting for service ${ARGOCD_REDIS_SERVICE_NAME}-announce-2 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP2=$(getent hosts "${ARGOCD_REDIS_SERVICE_NAME}-announce-2" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP2" ]; then
    echo "Could not resolve the announce ip for ${ARGOCD_REDIS_SERVICE_NAME}-announce-2"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE2/$ANNOUNCE_IP2/" "$HAPROXY_CONF"

auth=$(cat /redis-initial-pass/admin.password)
sed -i "s/replace-with-redis-auth/$auth/" "$HAPROXY_CONF"

