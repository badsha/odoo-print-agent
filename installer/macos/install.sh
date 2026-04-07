#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  echo "run as root"
  exit 1
fi

APP_DIR="/Applications/OdooPrintAgent"
CFG_DIR="/Library/Application Support/OdooPrintAgent"
LOG_DIR="/Library/Logs/OdooPrintAgent"
PLIST="/Library/LaunchDaemons/com.odoo.printagent.plist"

read -r -p "Odoo URL: " ODOO_URL
read -r -p "API Key: " API_KEY

mkdir -p "$APP_DIR" "$CFG_DIR" "$LOG_DIR"
install -m 0755 ./odoo-print-agent "$APP_DIR/odoo-print-agent"
if [[ ! -f "$CFG_DIR/config.json" ]]; then
  install -m 0600 ./config.json "$CFG_DIR/config.json"
fi

"$APP_DIR/odoo-print-agent" configure --config "$CFG_DIR/config.json" --odoo-url "$ODOO_URL" --api-key "$API_KEY"

install -m 0644 ./installer/macos/com.odoo.printagent.plist "$PLIST"
launchctl unload "$PLIST" >/dev/null 2>&1 || true
launchctl load "$PLIST"

echo "installed"
