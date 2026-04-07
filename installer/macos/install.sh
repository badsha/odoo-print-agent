#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  echo "run as root"
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

APP_DIR="/Applications/OdooPrintAgent"
CFG_DIR="/Library/Application Support/OdooPrintAgent"
LOG_DIR="/Library/Logs/OdooPrintAgent"
PLIST="/Library/LaunchDaemons/com.odoo.printagent.plist"

read -r -p "Odoo URL: " ODOO_URL
read -r -p "API Key: " API_KEY

mkdir -p "$APP_DIR" "$CFG_DIR" "$LOG_DIR"
BIN="$ROOT/odoo-print-agent"
if [[ ! -f "$BIN" ]]; then
  BIN="$ROOT/dist/darwin/odoo-print-agent"
fi
if [[ ! -f "$BIN" ]]; then
  echo "missing binary: $ROOT/dist/darwin/odoo-print-agent"
  echo "build it first: ./scripts/build.sh"
  exit 1
fi

install -m 0755 "$BIN" "$APP_DIR/odoo-print-agent"
if [[ ! -f "$CFG_DIR/config.json" ]]; then
  install -m 0600 "$ROOT/config.json" "$CFG_DIR/config.json"
fi

"$APP_DIR/odoo-print-agent" setup --config "$CFG_DIR/config.json" --odoo-url "$ODOO_URL" --api-key "$API_KEY" --log-file "$LOG_DIR/agent.jsonl" --log-level info --test-print

install -m 0644 "$ROOT/installer/macos/com.odoo.printagent.plist" "$PLIST"
launchctl unload "$PLIST" >/dev/null 2>&1 || true
launchctl load "$PLIST"

echo "installed"
