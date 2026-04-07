#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
  echo "run as root"
  exit 1
fi

PREFIX="/opt/odoo-print-agent"
ETC_DIR="/etc/odoo-print-agent"
LOG_DIR="/var/log/odoo-print-agent"

read -r -p "Odoo URL: " ODOO_URL
read -r -p "API Key: " API_KEY

mkdir -p "$PREFIX" "$ETC_DIR" "$LOG_DIR"
install -m 0755 ./odoo-print-agent "$PREFIX/odoo-print-agent"
if [[ ! -f "$ETC_DIR/config.json" ]]; then
  install -m 0600 ./config.json "$ETC_DIR/config.json"
fi

"$PREFIX/odoo-print-agent" setup --config "$ETC_DIR/config.json" --odoo-url "$ODOO_URL" --api-key "$API_KEY" --log-file "$LOG_DIR/agent.jsonl" --log-level info --test-print

install -m 0644 ./installer/linux/odoo-print-agent.service /etc/systemd/system/odoo-print-agent.service
systemctl daemon-reload
systemctl enable --now odoo-print-agent.service

echo "installed"
