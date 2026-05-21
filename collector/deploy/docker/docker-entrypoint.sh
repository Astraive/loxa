#!/bin/sh
set -e

if [ -n "$LOXA_CONFIG" ]; then
    echo "$LOXA_CONFIG" > /app/config.yaml
fi

if [ -n "$LOXA_CONFIG_FILE" ] && [ -f "$LOXA_CONFIG_FILE" ]; then
    cp "$LOXA_CONFIG_FILE" /app/config.yaml
fi

exec "$@"