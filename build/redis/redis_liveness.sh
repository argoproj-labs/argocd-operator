#!/usr/bin/env sh
# Minimalistic sh strict-mode as there is no bash in redis container
set -eu

response=$(redis-cli -a "${AUTH}" --no-auth-warning "$@" ping)
if [ "$response" != "PONG" ] && [ "${response:0:7}" != "LOADING" ] ; then
  echo "$response"
  exit 1
fi
echo "response=$response"
