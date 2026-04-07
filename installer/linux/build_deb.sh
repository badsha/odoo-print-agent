#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.1.0}"
ARCH="${ARCH:-amd64}"
OUT_DIR="${OUT_DIR:-$ROOT/dist/linux}"

BIN="${BIN:-$ROOT/dist/linux/odoo-print-agent}"

if [[ ! -f "$BIN" ]]; then
  echo "missing binary: $BIN"
  echo "build it first: ./scripts/build.sh"
  exit 1
fi

PKGROOT="$(mktemp -d)"
DEBIAN="$PKGROOT/DEBIAN"

mkdir -p "$OUT_DIR"

mkdir -p "$DEBIAN"
mkdir -p "$PKGROOT/opt/odoo-print-agent"
mkdir -p "$PKGROOT/etc/odoo-print-agent"
mkdir -p "$PKGROOT/etc/systemd/system"

install -m 0755 "$BIN" "$PKGROOT/opt/odoo-print-agent/odoo-print-agent"
install -m 0600 "$ROOT/config.json" "$PKGROOT/etc/odoo-print-agent/config.json"
install -m 0644 "$ROOT/installer/linux/odoo-print-agent.service" "$PKGROOT/etc/systemd/system/odoo-print-agent.service"

cat >"$DEBIAN/control" <<EOF
Package: odoo-print-agent
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: LL Print Platform
Description: Odoo Print Agent (LL Print Platform) - local print service for Odoo
EOF

cat >"$DEBIAN/conffiles" <<EOF
/etc/odoo-print-agent/config.json
EOF

cat >"$DEBIAN/postinst" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
fi

CFG="/etc/odoo-print-agent/config.json"
BIN="/opt/odoo-print-agent/odoo-print-agent"

if [[ -t 0 ]] && [[ -x "$BIN" ]]; then
  echo
  echo "Odoo Print Agent setup"
  read -r -p "Odoo URL: " ODOO_URL
  read -r -p "API Key: " API_KEY
  if [[ -n "${ODOO_URL:-}" ]] && [[ -n "${API_KEY:-}" ]]; then
    "$BIN" setup --config "$CFG" --odoo-url "$ODOO_URL" --api-key "$API_KEY" --test-print || true
  fi
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable --now odoo-print-agent.service || true
  fi
else
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable odoo-print-agent.service || true
  fi
fi

exit 0
EOF

cat >"$DEBIAN/prerm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop odoo-print-agent.service || true
  systemctl disable odoo-print-agent.service || true
fi

exit 0
EOF

chmod 0755 "$DEBIAN/postinst" "$DEBIAN/prerm"

dpkg-deb --build "$PKGROOT" "$OUT_DIR/odoo-print-agent_${VERSION}_${ARCH}.deb"
echo "built: $OUT_DIR/odoo-print-agent_${VERSION}_${ARCH}.deb"

