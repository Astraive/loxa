#!/bin/sh
set -e

if [ -n "$LOZA_CONFIG" ]; then
    echo "$LOZA_CONFIG" > /app/config.yaml
fi

if [ -n "$LOZA_CONFIG_FILE" ] && [ -f "$LOZA_CONFIG_FILE" ]; then
    cp "$LOZA_CONFIG_FILE" /app/config.yaml
fi

exec "$@"