echo "$(date) Start..."
HOSTNAME="$(cat /proc/sys/kernel/hostname)"
INDEX="${HOSTNAME##*-}"
SENTINEL_PORT={{- if eq .UseTLS "false" -}}26379{{- else -}}0{{- end }}
MASTER=''
MASTER_GROUP="argocd"
QUORUM="2"
REDIS_CONF=/data/conf/redis.conf
{{- if eq .UseTLS "false"}}
REDIS_PORT=6379
REDIS_TLS_PORT=
{{- else}}
REDIS_PORT=0
REDIS_TLS_PORT=6379
{{- end}}
SENTINEL_CONF=/data/conf/sentinel.conf
SENTINEL_TLS_PORT={{- if eq .UseTLS "true" -}}26379{{- end }}
SERVICE={{.ServiceName}}
SENTINEL_TLS_REPLICATION_ENABLED={{.UseTLS}}
REDIS_TLS_REPLICATION_ENABLED={{.UseTLS}}
set -eu

sentinel_get_master() {
set +e
    if [ "$SENTINEL_PORT" -eq 0 ]; then
        timeout 3 redis-cli -h "${SERVICE}" -p "${SENTINEL_TLS_PORT}" --tls --cacert /app/config/redis/tls/tls.crt sentinel get-master-addr-by-name "${MASTER_GROUP}" |\
        grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}'
    else
        timeout 3 redis-cli -h "${SERVICE}" -p "${SENTINEL_PORT}"  sentinel get-master-addr-by-name "${MASTER_GROUP}" |\
        grep -E '[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}'
    fi
set -e
}

sentinel_get_master_retry() {
    master=''
    retry=${1}
    sleep=3
    for i in $(seq 1 "${retry}"); do
        master=$(sentinel_get_master)
        if [ -n "${master}" ]; then
            break
        fi
        sleep $((sleep + i))
    done
    echo "${master}"
}

identify_master() {
    echo "Identifying redis master (get-master-addr-by-name).."
    echo "  using sentinel ({{.ServiceName}}), sentinel group name (argocd)"
    echo "  $(date).."
    MASTER="$(sentinel_get_master_retry 3)"
    if [ -n "${MASTER}" ]; then
        echo "  $(date) Found redis master (${MASTER})"
    else
        echo "  $(date) Did not find redis master (${MASTER})"
    fi
}

sentinel_update() {
    echo "Updating sentinel config.."
    echo "  evaluating sentinel id (\${SENTINEL_ID_${INDEX}})"
    eval MY_SENTINEL_ID="\$SENTINEL_ID_${INDEX}"
    echo "  sentinel id (${MY_SENTINEL_ID}), sentinel grp (${MASTER_GROUP}), quorum (${QUORUM})"
    sed -i "1s/^/sentinel myid ${MY_SENTINEL_ID}\\n/" "${SENTINEL_CONF}"
    if [ "$SENTINEL_TLS_REPLICATION_ENABLED" = true ]; then
        echo "  redis master (${1}:${REDIS_TLS_PORT})"
        sed -i "2s/^/sentinel monitor ${MASTER_GROUP} ${1} ${REDIS_TLS_PORT} ${QUORUM} \\n/" "${SENTINEL_CONF}"
    else
        echo "  redis master (${1}:${REDIS_PORT})"
        sed -i "2s/^/sentinel monitor ${MASTER_GROUP} ${1} ${REDIS_PORT} ${QUORUM} \\n/" "${SENTINEL_CONF}"
    fi
    echo "sentinel announce-ip ${ANNOUNCE_IP}" >> ${SENTINEL_CONF}
    if [ "$SENTINEL_PORT" -eq 0 ]; then
        echo "  announce (${ANNOUNCE_IP}:${SENTINEL_TLS_PORT})"
        echo "sentinel announce-port ${SENTINEL_TLS_PORT}" >> ${SENTINEL_CONF}
    else
        echo "  announce (${ANNOUNCE_IP}:${SENTINEL_PORT})"
        echo "sentinel announce-port ${SENTINEL_PORT}" >> ${SENTINEL_CONF}
    fi
}

redis_update() {
    echo "Updating redis config.."
    if [ "$REDIS_TLS_REPLICATION_ENABLED" = true ]; then
        echo "  we are slave of redis master (${1}:${REDIS_TLS_PORT})"
        echo "slaveof ${1} ${REDIS_TLS_PORT}" >> "${REDIS_CONF}"
        echo "slave-announce-port ${REDIS_TLS_PORT}" >> ${REDIS_CONF}
    else
        echo "  we are slave of redis master (${1}:${REDIS_PORT})"
        echo "slaveof ${1} ${REDIS_PORT}" >> "${REDIS_CONF}"
        echo "slave-announce-port ${REDIS_PORT}" >> ${REDIS_CONF}
    fi
    echo "slave-announce-ip ${ANNOUNCE_IP}" >> ${REDIS_CONF}
}

copy_config() {
    echo "Copying default redis config.."
    echo "  to '${REDIS_CONF}'"
    cp /readonly-config/redis.conf "${REDIS_CONF}"
    echo "Copying default sentinel config.."
    echo "  to '${SENTINEL_CONF}'"
    cp /readonly-config/sentinel.conf "${SENTINEL_CONF}"
}

setup_defaults() {
    echo "Setting up defaults.."
    echo "  using statefulset index (${INDEX})"
    if [ "${INDEX}" = "0" ]; then
        echo "Setting this pod as master for redis and sentinel.."
        echo "  using announce (${ANNOUNCE_IP})"
        redis_update "${ANNOUNCE_IP}"
        sentinel_update "${ANNOUNCE_IP}"
        echo "  make sure ${ANNOUNCE_IP} is not a slave (slaveof no one)"
        sed -i "s/^.*slaveof.*//" "${REDIS_CONF}"
    else
        echo "Getting redis master ip.."
        echo "  blindly assuming (${SERVICE}-announce-0) or (${SERVICE}-server-0) are master"
        DEFAULT_MASTER="$(getent_hosts 0 | awk '{ print $1 }')"
        echo "  identified redis (may be redis master) ip (${DEFAULT_MASTER})"
        if [ -z "${DEFAULT_MASTER}" ]; then
            echo "Error: Unable to resolve redis master (getent hosts)."
            exit 1
        fi
        echo "Setting default slave config for redis and sentinel.."
        echo "  using master ip (${DEFAULT_MASTER})"
        redis_update "${DEFAULT_MASTER}"
        sentinel_update "${DEFAULT_MASTER}"
    fi
}

redis_ping() {
set +e
    AUTH="$(cat /redis-initial-pass/admin.password)"
    if [ "$REDIS_PORT" -eq 0 ]; then
        env REDISCLI_AUTH="${AUTH}" redis-cli -h "${MASTER}" -p "${REDIS_TLS_PORT}" --tls --cacert /app/config/redis/tls/tls.crt ping
    else
        env REDISCLI_AUTH="${AUTH}" redis-cli -h "${MASTER}" -p "${REDIS_PORT}" ping
    fi
set -e
}

redis_ping_retry() {
    ping=''
    retry=${1}
    sleep=3
    for i in $(seq 1 "${retry}"); do
        if [ "$(redis_ping)" = "PONG" ]; then
           ping='PONG'
           break
        fi
        sleep $((sleep + i))
        MASTER=$(sentinel_get_master)
    done
    echo "${ping}"
}

find_master() {
    echo "Verifying redis master.."
    if [ "$REDIS_PORT" -eq 0 ]; then
        echo "  ping (${MASTER}:${REDIS_TLS_PORT})"
    else
        echo "  ping (${MASTER}:${REDIS_PORT})"
    fi
    echo "  $(date).."
    if [ "$(redis_ping_retry 3)" != "PONG" ]; then
        echo "  $(date) Can't ping redis master (${MASTER})"
        echo "Attempting to force failover (sentinel failover).."

        if [ "$SENTINEL_PORT" -eq 0 ]; then
            echo "  on sentinel (${SERVICE}:${SENTINEL_TLS_PORT}), sentinel grp (${MASTER_GROUP})"
            echo "  $(date).."
            if redis-cli -h "${SERVICE}" -p "${SENTINEL_TLS_PORT}" --tls --cacert /app/config/redis/tls/tls.crt sentinel failover "${MASTER_GROUP}" | grep -q 'NOGOODSLAVE' ; then
                echo "  $(date) Failover returned with 'NOGOODSLAVE'"
                echo "Setting defaults for this pod.."
                setup_defaults
                return 0
            fi
        else
            echo "  on sentinel (${SERVICE}:${SENTINEL_PORT}), sentinel grp (${MASTER_GROUP})"
            echo "  $(date).."
            if redis-cli -h "${SERVICE}" -p "${SENTINEL_PORT}" sentinel failover "${MASTER_GROUP}" | grep -q 'NOGOODSLAVE'; then
                echo "  $(date) Failover returned with 'NOGOODSLAVE'"
                echo "Setting defaults for this pod.."
                setup_defaults
                return 0
            fi
        fi

        echo "Hold on for 10sec"
        sleep 10
        echo "We should get redis master's ip now. Asking (get-master-addr-by-name).."
        if [ "$SENTINEL_PORT" -eq 0 ]; then
            echo "  sentinel (${SERVICE}:${SENTINEL_TLS_PORT}), sentinel grp (${MASTER_GROUP})"
        else
            echo "  sentinel (${SERVICE}:${SENTINEL_PORT}), sentinel grp (${MASTER_GROUP})"
        fi
        echo "  $(date).."
        MASTER="$(sentinel_get_master)"
        if [ "${MASTER}" ]; then
            echo "  $(date) Found redis master (${MASTER})"
            echo "Updating redis and sentinel config.."
            sentinel_update "${MASTER}"
            redis_update "${MASTER}"
        else
            echo "$(date) Error: Could not failover, exiting..."
            exit 1
        fi
    else
        echo "  $(date) Found reachable redis master (${MASTER})"
        echo "Updating redis and sentinel config.."
        sentinel_update "${MASTER}"
        redis_update "${MASTER}"
    fi
}

redis_ro_update() {
    echo "Updating read-only redis config.."
    echo "  redis.conf set 'replica-priority 0'"
    echo "replica-priority 0" >> ${REDIS_CONF}
}

getent_hosts() {
    index=${1:-${INDEX}}
    service="${SERVICE}-announce-${index}"
    pod="${SERVICE}-server-${index}"
    host=$(getent hosts "${service}")
    if [ -z "${host}" ]; then
        host=$(getent hosts "${pod}")
    fi
    echo "${host}"
}

mkdir -p /data/conf/

echo "Initializing config.."
copy_config

# where is redis master
identify_master

echo "Identify announce ip for this pod.."
echo "  using (${SERVICE}-announce-${INDEX}) or (${SERVICE}-server-${INDEX})"
ANNOUNCE_IP=$(getent_hosts | awk '{ print $1 }')
echo "  identified announce (${ANNOUNCE_IP})"
if [ -z "${ANNOUNCE_IP}" ]; then
    "Error: Could not resolve the announce ip for this pod."
    exit 1
elif [ "${MASTER}" ]; then
    find_master
else
    setup_defaults
fi

AUTH="$(cat /redis-initial-pass/admin.password)"
if [ -z "${AUTH}" ]; then
    echo "Error: Redis password not mounted correctly"
    exit 1
fi
echo "Setting redis auth values.."
ESCAPED_AUTH=$(echo "${AUTH}" | sed -e 's/[\/&]/\\&/g');
sed -i "s/__REPLACE_DEFAULT_AUTH__/${ESCAPED_AUTH}/" "${REDIS_CONF}" "${SENTINEL_CONF}"

if [ "${SENTINELAUTH:-}" ]; then
    echo "Setting sentinel auth values"
    ESCAPED_AUTH_SENTINEL=$(echo "$SENTINELAUTH" | sed -e 's/[\/&]/\\&/g');
    sed -i "s/__REPLACE_DEFAULT_SENTINEL_AUTH__/${ESCAPED_AUTH_SENTINEL}/" "$SENTINEL_CONF"
fi

echo "$(date) Ready..."
