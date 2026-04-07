package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type uiPageData struct {
	ConfigPath         string
	Addr               string
	Config             *Config
	OSPrinters         []string
	SelectedOSPrinters map[string]bool
	Message            string
	DoctorLines        []string
}

func uiCmd(args []string) {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	addr := fs.String("addr", "127.0.0.1:18085", "Listen address")
	timeout := fs.Duration("timeout", 8*time.Second, "Doctor HTTP/TCP timeout")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	if cfg, err := LoadConfig(absPath); err == nil {
		initLogging(cfg)
	}

	tmpl := template.Must(template.New("page").Parse(uiHTML))

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := LoadConfig(absPath)
		if err != nil {
			http.Error(w, "load config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		osPrinters, _ := ListOSPrinters()
		selectedOS := make(map[string]bool, len(cfg.Printers)*2)
		for _, p := range cfg.Printers {
			if strings.TrimSpace(p.OSPrinterName) != "" {
				selectedOS[strings.TrimSpace(p.OSPrinterName)] = true
			}
			if strings.TrimSpace(p.Name) != "" {
				selectedOS[strings.TrimSpace(p.Name)] = true
			}
		}
		data := uiPageData{
			ConfigPath:         absPath,
			Addr:               *addr,
			Config:             cfg,
			OSPrinters:         osPrinters,
			SelectedOSPrinters: selectedOS,
			Message:            strings.TrimSpace(r.URL.Query().Get("msg")),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, data)
	})

	mux.HandleFunc("/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg, err := LoadConfig(absPath)
		if err != nil {
			http.Error(w, "load config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
			return
		}
		selected := r.Form["os_printer"]
		var cleaned []string
		seen := make(map[string]struct{}, len(selected))
		for _, p := range selected {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			cleaned = append(cleaned, p)
		}
		if len(cleaned) == 0 {
			http.Redirect(w, r, "/?msg="+url.QueryEscape("No OS printers selected."), http.StatusSeeOther)
			return
		}
		sort.Strings(cleaned)
		cfg.Printers = buildPrinterConfigs(cleaned)
		if err := cfg.Save(absPath); err != nil {
			http.Error(w, "save config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/?msg="+url.QueryEscape(fmt.Sprintf("Imported %d printers from OS detection.", len(cfg.Printers))), http.StatusSeeOther)
	})

	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg, err := LoadConfig(absPath)
		if err != nil {
			http.Error(w, "load config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		cfg.OdooURL = strings.TrimSpace(r.FormValue("odoo_url"))
		cfg.APIKey = strings.TrimSpace(r.FormValue("api_key"))
		cfg.SpoolDir = strings.TrimSpace(r.FormValue("spool_dir"))
		cfg.PollIntervalSeconds = parseInt(r.FormValue("poll_interval_seconds"), cfg.PollIntervalSeconds)
		cfg.LeaseSeconds = parseInt(r.FormValue("lease_seconds"), cfg.LeaseSeconds)
		cfg.Limit = parseInt(r.FormValue("limit"), cfg.Limit)

		for i := range cfg.Printers {
			cfg.Printers[i].OSPrinterName = strings.TrimSpace(r.FormValue(fmt.Sprintf("printer_%d_os_printer_name", i)))
			cfg.Printers[i].NetworkHost = strings.TrimSpace(r.FormValue(fmt.Sprintf("printer_%d_network_host", i)))
			cfg.Printers[i].NetworkPort = parseInt(r.FormValue(fmt.Sprintf("printer_%d_network_port", i)), cfg.Printers[i].NetworkPort)
		}

		if err := cfg.Save(absPath); err != nil {
			http.Error(w, "save config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/?msg="+url.QueryEscape("Saved config."), http.StatusSeeOther)
	})

	mux.HandleFunc("/doctor", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := LoadConfig(absPath)
		if err != nil {
			http.Error(w, "load config: "+err.Error(), http.StatusInternalServerError)
			return
		}
		osPrinters, _ := ListOSPrinters()
		ctx, cancel := context.WithTimeout(r.Context(), *timeout)
		defer cancel()
		lines := uiDoctorReport(ctx, cfg, absPath, *timeout)
		data := uiPageData{
			ConfigPath:  absPath,
			Addr:        *addr,
			Config:      cfg,
			OSPrinters:  osPrinters,
			DoctorLines: lines,
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, data)
	})

	mux.HandleFunc("/api/os_printers", func(w http.ResponseWriter, r *http.Request) {
		printers, err := ListOSPrinters()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"printers": printers})
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logInfo("ui_listen", "", map[string]any{"config": absPath, "addr": "http://" + *addr})
	logFatalf("%v", srv.ListenAndServe())
}

func parseInt(v string, def int) int {
	s := strings.TrimSpace(v)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func uiDoctorReport(ctx context.Context, cfg *Config, configPath string, timeout time.Duration) []string {
	lines := []string{
		"config: " + configPath,
		"odoo_url: " + strings.TrimSpace(cfg.OdooURL),
	}

	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.OdooURL), "/"))
	if err != nil || strings.TrimSpace(baseURL.Scheme) == "" || strings.TrimSpace(baseURL.Host) == "" {
		lines = append(lines, fmt.Sprintf("odoo: fail: invalid odoo_url %q", cfg.OdooURL))
		return lines
	}

	httpClient := &http.Client{Timeout: timeout}

	if err := doctorCheckOdooReachable(ctx, httpClient, baseURL); err != nil {
		lines = append(lines, "odoo: fail: "+err.Error())
		return lines
	}
	lines = append(lines, "odoo: ok")

	apiInstalled, err := doctorCheckPrintAPIInstalled(ctx, httpClient, baseURL)
	if err != nil {
		lines = append(lines, "api: fail: "+err.Error())
		return lines
	}
	if !apiInstalled {
		lines = append(lines, "api: missing: install the Odoo module ll_print_platform")
		return lines
	}
	lines = append(lines, "api: ok")

	if err := doctorCheckAPIKey(ctx, httpClient, baseURL, strings.TrimSpace(cfg.APIKey)); err != nil {
		lines = append(lines, "api_key: fail: "+err.Error())
		lines = append(lines, "hint: In Odoo: Printing → Configuration → Printing Setup → Generate / Load API Key (ensure agent is active)")
		return lines
	}
	lines = append(lines, "api_key: ok")

	lines = append(lines, uiDoctorPrinters(cfg.Printers)...)
	return lines
}

func uiDoctorPrinters(printers []PrinterConfig) []string {
	if len(printers) == 0 {
		return []string{"printers: none configured"}
	}

	lines := []string{fmt.Sprintf("printers: %d configured", len(printers))}
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if _, err := exec.LookPath("lp"); err != nil {
			lines = append(lines, "cups: warn: lp not found in PATH")
		}
		if _, err := exec.LookPath("lpstat"); err != nil {
			lines = append(lines, "cups: warn: lpstat not found in PATH")
		}
	}

	osPrinters, err := ListOSPrinters()
	if err != nil {
		lines = append(lines, "os_printers: warn: "+err.Error())
		osPrinters = nil
	}
	known := make(map[string]struct{}, len(osPrinters))
	for _, p := range osPrinters {
		known[p] = struct{}{}
	}

	var missingOS []string
	for _, p := range printers {
		if strings.TrimSpace(p.OSPrinterName) != "" {
			if _, ok := known[strings.TrimSpace(p.OSPrinterName)]; !ok && len(known) > 0 {
				missingOS = append(missingOS, fmt.Sprintf("%s -> %s", p.AgentIdentifier, strings.TrimSpace(p.OSPrinterName)))
			}
		}
		if strings.TrimSpace(p.NetworkHost) != "" {
			host := strings.TrimSpace(p.NetworkHost)
			port := p.NetworkPort
			if port <= 0 {
				port = 9100
			}
			addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
			conn, err := (&net.Dialer{Timeout: 2 * time.Second}).Dial("tcp", addr)
			if err != nil {
				lines = append(lines, "network: warn: "+p.AgentIdentifier+" dial failed: "+err.Error())
			} else {
				_ = conn.Close()
			}
		}
		if strings.TrimSpace(p.OSPrinterName) == "" && strings.TrimSpace(p.NetworkHost) == "" {
			lines = append(lines, "mapping: info: "+p.AgentIdentifier+" will spool to disk (no os_printer_name/network_host)")
		}
	}

	if len(missingOS) > 0 {
		sort.Strings(missingOS)
		lines = append(lines, "mapping: warn: unknown os_printer_name values:")
		for _, m := range missingOS {
			lines = append(lines, "- "+m)
		}
		lines = append(lines, "hint: Run: odoo-print-agent printers")
	}

	return lines
}

const uiHTML = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Odoo Print Agent Setup</title>
  <style>
    :root { color-scheme: light; }
    body { font-family: -apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Helvetica, Arial, sans-serif; margin: 24px; background: #fff; color: #111; }
    h1 { font-size: 20px; margin: 0 0 16px; }
    .muted { color: #666; }
    .box { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin: 12px 0; background: #fff; }
    label { display: block; font-size: 12px; margin-bottom: 4px; color: #333; }
    input[type=text], input[type=password], input[type=number] { width: 100%; padding: 8px 10px; border: 1px solid #ccc; border-radius: 6px; }
    select { width: 100%; padding: 8px 10px; border: 1px solid #ccc; border-radius: 6px; background: white; }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .row3 { display: grid; grid-template-columns: 2fr 1fr 1fr; gap: 12px; }
    .btn { display: inline-block; padding: 10px 14px; border-radius: 8px; border: 1px solid #111; background: #111; color: #fff; text-decoration: none; cursor: pointer; }
    .btn.secondary { background: #fff; color: #111; }
    .msg { padding: 10px 12px; border-radius: 8px; background: #f1f5f9; }
    pre { white-space: pre-wrap; padding: 12px; border-radius: 8px; background: #0b1020; color: #d1e7ff; }
    table { width: 100%; border-collapse: collapse; }
    th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid #eee; vertical-align: top; }
    th { font-size: 12px; color: #666; }
    .small { font-size: 12px; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-top: 10px; }
    .check { display: flex; gap: 10px; align-items: center; padding: 6px 8px; border: 1px solid #eee; border-radius: 8px; background: #fafafa; }
    .check span { color: #111; font-size: 13px; }
    input[type=checkbox] { width: 16px; height: 16px; }
  </style>
</head>
<body>
  <h1>Odoo Print Agent Setup</h1>
  <div class="muted small">Config: {{.ConfigPath}} · UI: http://{{.Addr}}</div>

  {{if .Message}}
    <div class="box msg">{{.Message}}</div>
  {{end}}

  <form method="post" action="/import">
    <div class="box">
      <div style="display:flex; gap:10px; align-items:center; justify-content:space-between;">
        <div>
          <div style="font-weight:600;">Detected OS Printers</div>
          <div class="muted small">Select printers to expose to Odoo. This replaces the configured printers list.</div>
        </div>
        <div style="display:flex; gap:10px;">
          <a class="btn secondary" href="/">Refresh</a>
          <button class="btn" type="submit">Import Selected</button>
        </div>
      </div>
      {{if .OSPrinters}}
        <div class="grid">
          {{range .OSPrinters}}
            <label class="check"><input type="checkbox" name="os_printer" value="{{.}}" {{if index $.SelectedOSPrinters .}}checked{{end}} /> <span class="small">{{.}}</span></label>
          {{end}}
        </div>
      {{else}}
        <div class="muted small" style="margin-top:10px;">No OS printers detected.</div>
      {{end}}
    </div>
  </form>

  <form method="post" action="/save">
    <div class="box">
      <div class="row">
        <div>
          <label>Odoo URL</label>
          <input type="text" name="odoo_url" value="{{.Config.OdooURL}}" placeholder="https://your-odoo.example.com" />
        </div>
        <div>
          <label>API Key</label>
          <input type="password" name="api_key" value="{{.Config.APIKey}}" placeholder="Paste from Odoo Printing Setup" />
        </div>
      </div>
      <div class="row3" style="margin-top: 12px;">
        <div>
          <label>Spool Dir</label>
          <input type="text" name="spool_dir" value="{{.Config.SpoolDir}}" />
        </div>
        <div>
          <label>Poll (sec)</label>
          <input type="number" name="poll_interval_seconds" value="{{.Config.PollIntervalSeconds}}" />
        </div>
        <div>
          <label>Lease (sec)</label>
          <input type="number" name="lease_seconds" value="{{.Config.LeaseSeconds}}" />
        </div>
      </div>
      <div class="row3" style="margin-top: 12px;">
        <div></div>
        <div>
          <label>Limit</label>
          <input type="number" name="limit" value="{{.Config.Limit}}" />
        </div>
        <div></div>
      </div>
    </div>

    <div class="box">
      <div style="display:flex; gap:10px; align-items:center; justify-content:space-between;">
        <div>
          <div style="font-weight:600;">Printer Mappings</div>
          <div class="muted small">Select the OS printer queue name for each configured printer.</div>
        </div>
        <div style="display:flex; gap:10px;">
          <a class="btn secondary" href="/doctor">Run Doctor</a>
          <button class="btn" type="submit">Save</button>
        </div>
      </div>

      <table style="margin-top: 10px;">
        <thead>
          <tr>
            <th>Agent Identifier</th>
            <th>Name</th>
            <th>OS Printer Name</th>
            <th>Network Host</th>
            <th>Port</th>
          </tr>
        </thead>
        <tbody>
          {{range $i, $p := .Config.Printers}}
          <tr>
            <td class="small">{{$p.AgentIdentifier}}</td>
            <td class="small">{{$p.Name}}</td>
            <td>
              <input type="text" name="printer_{{$i}}_os_printer_name" value="{{$p.OSPrinterName}}" list="os_printers" placeholder="Pick from list…" />
            </td>
            <td>
              <input type="text" name="printer_{{$i}}_network_host" value="{{$p.NetworkHost}}" placeholder="Optional (raw TCP)" />
            </td>
            <td style="width: 90px;">
              <input type="number" name="printer_{{$i}}_network_port" value="{{$p.NetworkPort}}" />
            </td>
          </tr>
          {{end}}
        </tbody>
      </table>
      {{if not .Config.Printers}}
        <div class="muted small" style="margin-top:10px;">No printers configured. Use “Detected OS Printers” above to import.</div>
      {{end}}

      <datalist id="os_printers">
        {{range .OSPrinters}}
          <option value="{{.}}"></option>
        {{end}}
      </datalist>
    </div>
  </form>

  {{if .DoctorLines}}
    <div class="box">
      <div style="font-weight:600; margin-bottom: 8px;">Doctor</div>
      <pre>{{range .DoctorLines}}{{.}}
{{end}}</pre>
      <a class="btn secondary" href="/">Back</a>
    </div>
  {{end}}
</body>
</html>`
