# odoo-print-agent

## Overview

This is the local agent for **LL Print Platform** (Odoo addon). It runs on a machine that has access to printers and connects to Odoo using an API key.

Current architecture (today):
- Syncs printers to Odoo (`/api/print/printers/sync`)
- Polls jobs from Odoo (`/api/print/jobs`)
- Marks jobs ack/done/fail (`/api/print/job/<id>/*`)
- Prints jobs locally using either:
  - OS printer queue (macOS/Linux via CUPS `lp`)
  - Raw TCP printing (LAN printers on port 9100, for raw/ESC-POS jobs)
  - Spool-to-file fallback (writes payloads to disk)

## Install (End Users)

Recommended (internal): download the binary for your OS and run the interactive installer command. It launches a local setup UI, writes `config.json`, and installs an auto-start service.

### Internal install (binary + UI)

macOS (per-user, starts on login):

```bash
./odoo-print-agent install
```

macOS (system-wide, starts on boot):

```bash
sudo ./odoo-print-agent install
```

Linux (systemd, starts on boot):

```bash
sudo ./odoo-print-agent install
```

Windows (run from an elevated terminal, starts as a Windows service):

```powershell
odoo-print-agent.exe install
```

The installer prints a `Setup UI: http://127.0.0.1:PORT/` URL and opens it in your browser (unless `--no-open` is used). Save Odoo URL, API key, and printers in the UI; then the agent service is started and set to auto-start.

Default install locations:
- macOS (per-user): `~/Library/Application Support/OdooPrintAgent/config.json`
- macOS (system): `/Library/Application Support/OdooPrintAgent/config.json`
- Linux: `/etc/odoo-print-agent/config.json`
- Windows: `C:\ProgramData\OdooPrintAgent\config.json`

Logs:
- macOS: `~/Library/Logs/OdooPrintAgent/agent.jsonl` or `/Library/Logs/OdooPrintAgent/agent.jsonl`
- Linux: `/var/log/odoo-print-agent/agent.jsonl`
- Windows: `C:\ProgramData\OdooPrintAgent\logs\agent.jsonl`

### Packaged installers (optional)

If you are building/distributing yourself, this repo also includes installer templates:

### Windows

- Inno Setup script: `installer/windows/print-agent.iss`
- Can install as a Windows Service or “Run on login” depending on your build settings
- Installer config path: `C:\ProgramData\OdooPrintAgent\config.json`

### macOS

- Installer script: `installer/macos/install.sh`
- Installs:
  - Binary: `/Applications/OdooPrintAgent/odoo-print-agent`
  - Config: `/Library/Application Support/OdooPrintAgent/config.json`
  - Logs: `/Library/Logs/OdooPrintAgent/agent.jsonl`
  - LaunchDaemon (auto-start on boot): `/Library/LaunchDaemons/com.odoo.printagent.plist`

### Linux

- Installer script: `installer/linux/install.sh`
- Installs:
  - Binary: `/opt/odoo-print-agent/odoo-print-agent`
  - Config: `/etc/odoo-print-agent/config.json`
  - systemd unit (auto-start on boot): `/etc/systemd/system/odoo-print-agent.service`

## Configure

Default config path:
- macOS: `~/Library/Application Support/odoo-print-agent/config.json`
- Linux: `~/.config/odoo-print-agent/config.json`
- Windows: `%AppData%\\odoo-print-agent\\config.json`

If a `./config.json` file exists in the current directory, the agent uses it by default (useful for development).

Edit the config file directly:
- `odoo_url`: base URL of your Odoo (must be reachable from the agent machine)
- `api_key`: API key from **Printing → Configuration → Printing Setup**
- `printers`: list of printers this agent exposes to Odoo

Or use the CLI:

```bash
go run . configure --odoo-url https://YOUR-ODOO-URL --api-key YOUR_API_KEY
```

Or use the local setup UI (writes `config.json` for you):

```bash
go run . ui
```

Notes:
- `ui` starts a local web server and keeps running while you use the browser UI. Stop it with Ctrl+C when you are done saving the config.
- `run` is the actual agent loop and should keep running (as a service in production).

## Doctor

Checks connectivity + common setup issues (Odoo reachable, module installed, API key valid, printer mapping):

```bash
go run . doctor
```

## List OS Printers

On macOS/Linux this lists CUPS queues (names used by `os_printer_name`):

```bash
go run . printers
```

## Printer Mapping

Odoo printers are created/updated from `printers[].agent_identifier`. To actually print on the agent machine, set one of:

- `os_printer_name` (macOS/Linux): prints through CUPS (`lp -d <name>`)
- `network_host` + optional `network_port` (default 9100): raw TCP printing (raw/escpos jobs only)

Notes:
- Windows uses `os_printer_name` as the Windows printer name:
  - PDF jobs print silently via SumatraPDF (installed/bundled by the Windows installer).
- If you set neither `os_printer_name` nor `network_host`, the agent will spool the job payload to disk (`spool_dir`) instead of printing.

Example:

```json
{
  "odoo_url": "http://localhost:8069",
  "api_key": "YOUR_KEY",
  "poll_interval_seconds": 3,
  "lease_seconds": 30,
  "limit": 20,
  "spool_dir": "spool",
  "printers": [
    {
      "agent_identifier": "counter_receipt",
      "name": "Counter Receipt",
      "printer_type": "receipt",
      "code": "R1",
      "os_printer_name": "HP_LaserJet"
    },
    {
      "agent_identifier": "kitchen_escpos",
      "name": "Kitchen",
      "printer_type": "kitchen",
      "code": "K1",
      "network_host": "192.168.1.50",
      "network_port": 9100
    }
  ]
}
```

## Run

```bash
go run . run
```

One cycle only:

```bash
go run . run --once
```

## Build

```bash
./scripts/build.sh
```

## Roadmap (Production Installer Wizard)

Target end-user experience:
- Installer wizard selects printers during setup (no JSON editing)
- Agent runs as a background service on boot (Windows service, LaunchDaemon, systemd)
- Silent PDF printing on Windows (SumatraPDF) and macOS/Linux (lp)
- Optional “direct /print HTTP service” mode for immediate printing (instead of polling)
