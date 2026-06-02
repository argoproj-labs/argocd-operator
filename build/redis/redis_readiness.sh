#!/usr/bin/env sh
# Minimalistic sh strict-mode as there is no bash in redis container
set -eu

redis_pwd="$(cat /app/config/redis-auth/auth)"
if [ -z "$redis_pwd" ]; then
    echo "Error: Redis password not mounted correctly"
    exit 1
fi

response=$(env REDISCLI_AUTH="${redis_pwd}" redis-cli --no-auth-warning "$@" ping)
if [ "$response" != "PONG" ]; then
  echo "$response"
  exit 1
fi
echo "response=$response"
