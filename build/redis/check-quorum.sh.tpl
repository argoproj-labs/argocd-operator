#!/bin/sh
set -eu
MASTER_GROUP="argocd"
SENTINEL_PORT=26379
REDIS_PORT=6379
NUM_SLAVES=$(redis-cli -p "$SENTINEL_PORT" sentinel master argocd | awk '/num-slaves/{getline; print}')
MIN_SLAVES=1

if [ "$1" = "$SENTINEL_PORT" ]; then
    if redis-cli -p "$SENTINEL_PORT" sentinel ckquorum "$MASTER_GROUP" | grep -q NOQUORUM ; then
        echo "ERROR: NOQUORUM. Sentinel quorum check failed, not enough sentinels found"
        exit 1
    fi
elif [ "$1" = "$REDIS_PORT" ]; then
    if [ "$MIN_SLAVES" -gt "$NUM_SLAVES" ]; then
        echo "Could not find enough replicating slaves. Needed $MIN_SLAVES but found $NUM_SLAVES"
        exit 1
    fi
fi
sh /probes/readiness.sh "$1"
