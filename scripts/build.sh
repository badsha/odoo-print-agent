#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.1.0}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || true)}"
BUILDDATE="${BUILDDATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILDDATE}"

mkdir -p dist/darwin dist/linux dist/windows

GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/darwin/odoo-print-agent .
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/darwin/odoo-print-agent-arm64 .
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/linux/odoo-print-agent .
GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/linux/odoo-print-agent-arm64 .
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/windows/odoo-print-agent.exe .
