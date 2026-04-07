# odoo-print-agent

## What it does

This is the local agent for **LL Print Platform**:

- Syncs printers to Odoo (`/api/print/printers/sync`)
- Polls jobs from Odoo (`/api/print/jobs`)
- Marks jobs ack/done/fail (`/api/print/job/<id>/*`)
- Prints jobs locally using either:
  - OS printer queue (macOS/Linux via CUPS `lp`)
  - Raw TCP printing (LAN printers on port 9100, for raw/ESC-POS jobs)
  - Spool-to-file fallback (writes payloads to disk)

## Configure

Default config path:
- macOS: `~/Library/Application Support/odoo-print-agent/config.json`
- Linux: `~/.config/odoo-print-agent/config.json`
- Windows: `%AppData%\\odoo-print-agent\\config.json`

If a `./config.json` file exists in the current directory, the agent uses it by default (useful for development).

Edit the config file:
- `odoo_url`: base URL of your Odoo (must be reachable from the agent machine)
- `api_key`: API key from **Printing → Configuration → Printing Setup**
- `printers`: list of printers this agent exposes to Odoo

Or use the CLI:

```bash
go run . configure --odoo-url https://YOUR-ODOO-URL --api-key YOUR_API_KEY
```

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
