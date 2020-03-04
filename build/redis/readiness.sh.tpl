#!/bin/sh
set -eu
CHECK_SERVER="$(redis-cli -p "$1" ping)"

if [ "$CHECK_SERVER" != "PONG" ]; then
    echo "Server check failed with: $CHECK_SERVER"
    exit 1
fi
