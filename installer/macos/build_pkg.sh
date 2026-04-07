#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.1.0}"
OUT_DIR="${OUT_DIR:-$ROOT/dist/macos}"

BIN="${BIN:-$ROOT/dist/darwin/odoo-print-agent}"
APP_SIGN_IDENTITY="${APP_SIGN_IDENTITY:-}"
PKG_SIGN_IDENTITY="${PKG_SIGN_IDENTITY:-}"
NOTARIZE="${NOTARIZE:-0}"
NOTARY_PROFILE="${NOTARY_PROFILE:-}"
APPLE_ID="${APPLE_ID:-}"
TEAM_ID="${TEAM_ID:-}"
APP_SPECIFIC_PASSWORD="${APP_SPECIFIC_PASSWORD:-}"

if [[ ! -f "$BIN" ]]; then
  echo "missing binary: $BIN"
  echo "build it first: ./scripts/build.sh"
  exit 1
fi

PKGROOT="$(mktemp -d)"
SCRIPTS="$ROOT/installer/macos/pkg/scripts"
IDENTIFIER="com.odoo.printagent"

mkdir -p "$OUT_DIR"
chmod +x "$SCRIPTS/postinstall" || true

mkdir -p "$PKGROOT/Applications/OdooPrintAgent"
mkdir -p "$PKGROOT/Library/Application Support/OdooPrintAgent"
mkdir -p "$PKGROOT/Library/LaunchDaemons"

if [[ -n "$APP_SIGN_IDENTITY" ]]; then
  codesign --force --options runtime --timestamp --sign "$APP_SIGN_IDENTITY" "$BIN"
fi

install -m 0755 "$BIN" "$PKGROOT/Applications/OdooPrintAgent/odoo-print-agent"
install -m 0644 "$ROOT/config.json" "$PKGROOT/Applications/OdooPrintAgent/config.default.json"
install -m 0644 "$ROOT/installer/macos/com.odoo.printagent.plist" "$PKGROOT/Library/LaunchDaemons/com.odoo.printagent.plist"

UNSIGNED_PKG="$OUT_DIR/OdooPrintAgent-$VERSION-unsigned.pkg"
OUT_PKG="$OUT_DIR/OdooPrintAgent-$VERSION.pkg"

pkgbuild \
  --root "$PKGROOT" \
  --scripts "$SCRIPTS" \
  --identifier "$IDENTIFIER" \
  --version "$VERSION" \
  "$UNSIGNED_PKG"

if [[ -n "$PKG_SIGN_IDENTITY" ]]; then
  productsign --sign "$PKG_SIGN_IDENTITY" "$UNSIGNED_PKG" "$OUT_PKG"
  rm -f "$UNSIGNED_PKG"
else
  mv -f "$UNSIGNED_PKG" "$OUT_PKG"
fi

if [[ "$NOTARIZE" == "1" ]]; then
  if [[ -n "$NOTARY_PROFILE" ]]; then
    xcrun notarytool submit "$OUT_PKG" --keychain-profile "$NOTARY_PROFILE" --wait
  else
    if [[ -z "$APPLE_ID" || -z "$TEAM_ID" || -z "$APP_SPECIFIC_PASSWORD" ]]; then
      echo "notarization requested but missing credentials"
      echo "set NOTARY_PROFILE or (APPLE_ID, TEAM_ID, APP_SPECIFIC_PASSWORD)"
      exit 1
    fi
    xcrun notarytool submit "$OUT_PKG" --apple-id "$APPLE_ID" --team-id "$TEAM_ID" --password "$APP_SPECIFIC_PASSWORD" --wait
  fi
  xcrun stapler staple "$OUT_PKG"
fi

echo "built: $OUT_PKG"
