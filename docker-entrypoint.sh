#!/bin/sh
# Container entrypoint for CLIProxyAPI.
# On first boot it seeds a minimal config.yaml onto the persistent volume
# (CPA_CONFIG_PATH, default /data/config.yaml) from environment variables,
# then starts the server. Later boots reuse the on-volume config, so changes
# made through the management API survive redeploys.
#
# Environment variables:
#   CPA_CONFIG_PATH      Config file path (default: /data/config.yaml)
#   CPA_AUTH_DIR         Auth/token directory (default: <config dir>/auths)
#   CPA_PORT             Listen port (default: 8317)
#   API_KEYS             Comma-separated client API keys seeded into api-keys
#   MANAGEMENT_PASSWORD  Remote management secret (read by the server itself)
set -e

CONFIG_PATH="${CPA_CONFIG_PATH:-/data/config.yaml}"
AUTH_DIR="${CPA_AUTH_DIR:-$(dirname "$CONFIG_PATH")/auths}"

if [ ! -f "$CONFIG_PATH" ]; then
  echo "[entrypoint] seeding $CONFIG_PATH from environment"
  mkdir -p "$(dirname "$CONFIG_PATH")" "$AUTH_DIR"

  {
    echo 'host: ""'
    echo "port: ${CPA_PORT:-8317}"
    echo 'remote-management:'
    echo '  allow-remote: true'
    echo "auth-dir: \"$AUTH_DIR\""
    if [ -n "$API_KEYS" ]; then
      echo 'api-keys:'
      OLD_IFS=$IFS
      IFS=','
      for key in $API_KEYS; do
        key=$(printf '%s' "$key" | tr -d '[:space:]')
        [ -n "$key" ] && printf '  - "%s"\n' "$key"
      done
      IFS=$OLD_IFS
    else
      echo 'api-keys: []'
    fi
    echo 'debug: false'
  } > "$CONFIG_PATH"
else
  echo "[entrypoint] using existing config at $CONFIG_PATH"
fi

exec ./CLIProxyAPI --config "$CONFIG_PATH" "$@"
