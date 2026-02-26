#!/usr/bin/env sh
# Minimalistic sh strict-mode as there is no bash in redis container
set -eu

response=$(redis-cli "$@" ping)
if [ "$response" != "PONG" ]; then
  echo "$response"
  exit 1
fi
echo "response=$response"
