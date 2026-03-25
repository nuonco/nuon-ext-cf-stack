#!/usr/bin/env bash

set -e
set -o pipefail
set -u


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

CONFIG_FILE="${NUON_CONFIG_FILE:-$HOME/.nuon}"

if [[ ! -f "$CONFIG_FILE" ]]; then
  echo "error: config file not found: $CONFIG_FILE" >&2
  exit 1
fi

# Read YAML-style key: value pairs from config and export as NUON_ env vars
while IFS=': ' read -r key value; do
  [[ -z "$key" || "$key" == \#* ]] && continue
  value="${value#"${value%%[![:space:]]*}"}"
  [[ -z "$value" || "$value" == '""' ]] && continue
  env_name="NUON_$(echo "$key" | tr '[:lower:]' '[:upper:]')"
  export "$env_name=$value"
done < "$CONFIG_FILE"

cd "$ROOT_DIR"
GOWORK=off go run . "$@"
