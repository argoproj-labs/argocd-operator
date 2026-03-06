HAPROXY_CONF=/data/haproxy.cfg
cp /readonly/haproxy.cfg "$HAPROXY_CONF"
for loop in $(seq 1 10); do
    getent hosts {{.ServiceName}}-announce-0 && break
    echo "Waiting for service {{.ServiceName}}-announce-0 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP0=$(getent hosts "{{.ServiceName}}-announce-0" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP0" ]; then
    echo "Could not resolve the announce ip for {{.ServiceName}}-announce-0"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE0/$ANNOUNCE_IP0/" "$HAPROXY_CONF"

for loop in $(seq 1 10); do
    getent hosts {{.ServiceName}}-announce-1 && break
    echo "Waiting for service {{.ServiceName}}-announce-1 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP1=$(getent hosts "{{.ServiceName}}-announce-1" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP1" ]; then
    echo "Could not resolve the announce ip for {{.ServiceName}}-announce-1"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE1/$ANNOUNCE_IP1/" "$HAPROXY_CONF"

for loop in $(seq 1 10); do
    getent hosts {{.ServiceName}}-announce-2 && break
    echo "Waiting for service {{.ServiceName}}-announce-2 to be ready ($loop) ..." && sleep 1
done
ANNOUNCE_IP2=$(getent hosts "{{.ServiceName}}-announce-2" | awk '{ print $1 }')
if [ -z "$ANNOUNCE_IP2" ]; then
    echo "Could not resolve the announce ip for {{.ServiceName}}-announce-2"
    exit 1
fi
sed -i "s/REPLACE_ANNOUNCE2/$ANNOUNCE_IP2/" "$HAPROXY_CONF"

AUTH="$(cat /app/config/redis-auth/auth)"
if [ -z "${AUTH}" ]; then
    echo "Error: Redis password not mounted correctly"
    exit 1
fi
echo "Setting redis auth values.."
ESCAPED_AUTH=$(echo "${AUTH}" | sed -e 's/[\/&]/\\&/g');
sed -i "s/__REPLACE_DEFAULT_AUTH__/${ESCAPED_AUTH}/" "$HAPROXY_CONF"

