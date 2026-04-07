package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

func doctorCmd(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	timeout := fs.Duration("timeout", 10*time.Second, "HTTP/TCP timeout")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	cfg, err := LoadConfig(absPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	fmt.Println("config:", absPath)
	fmt.Println("odoo_url:", strings.TrimSpace(cfg.OdooURL))

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	baseURL, err := url.Parse(strings.TrimRight(strings.TrimSpace(cfg.OdooURL), "/"))
	if err != nil || strings.TrimSpace(baseURL.Scheme) == "" || strings.TrimSpace(baseURL.Host) == "" {
		log.Fatalf("invalid odoo_url: %q", cfg.OdooURL)
	}

	httpClient := &http.Client{Timeout: *timeout}

	if err := doctorCheckOdooReachable(ctx, httpClient, baseURL); err != nil {
		fmt.Println("odoo:", "fail:", err.Error())
		os.Exit(1)
	}
	fmt.Println("odoo:", "ok")

	apiInstalled, err := doctorCheckPrintAPIInstalled(ctx, httpClient, baseURL)
	if err != nil {
		fmt.Println("api:", "fail:", err.Error())
		os.Exit(1)
	}
	if !apiInstalled {
		fmt.Println("api:", "missing: install the Odoo module ll_print_platform")
		os.Exit(1)
	}
	fmt.Println("api:", "ok")

	if err := doctorCheckAPIKey(ctx, httpClient, baseURL, strings.TrimSpace(cfg.APIKey)); err != nil {
		fmt.Println("api_key:", "fail:", err.Error())
		fmt.Println("hint:", "In Odoo: Printing → Configuration → Printing Setup → Generate / Load API Key (ensure agent is active)")
		os.Exit(1)
	}
	fmt.Println("api_key:", "ok")

	doctorCheckPrinters(cfg.Printers)
}

func doctorCheckOdooReachable(ctx context.Context, c *http.Client, base *url.URL) error {
	u := *base
	u.Path = "/web/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 500 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("http %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func doctorCheckPrintAPIInstalled(ctx context.Context, c *http.Client, base *url.URL) (bool, error) {
	u := *base
	u.Path = "/api/print/jobs"
	q := make(url.Values)
	q.Set("limit", "1")
	q.Set("lease_seconds", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<10))
	if res.StatusCode == 404 && looksLikeHTML(body) {
		return false, nil
	}
	if res.StatusCode == 401 {
		return true, nil
	}
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return true, nil
	}
	return false, fmt.Errorf("unexpected http %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
}

func doctorCheckAPIKey(ctx context.Context, c *http.Client, base *url.URL, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("missing api_key in config")
	}
	u := *base
	u.Path = "/api/print/printers/sync"

	body := map[string]any{"printers": []any{}}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	respBytes, _ := io.ReadAll(io.LimitReader(res.Body, 32<<10))
	if res.StatusCode == 404 && looksLikeHTML(respBytes) {
		return fmt.Errorf("print API not found (ll_print_platform not installed)")
	}
	if res.StatusCode == 401 {
		return fmt.Errorf("unauthorized (api key not recognized by Odoo)")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", res.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	return nil
}

func doctorCheckPrinters(printers []PrinterConfig) {
	if len(printers) == 0 {
		fmt.Println("printers:", "none configured")
		return
	}

	fmt.Println("printers:", len(printers), "configured")

	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if _, err := exec.LookPath("lp"); err != nil {
			fmt.Println("cups:", "warn: lp not found in PATH")
		}
		if _, err := exec.LookPath("lpstat"); err != nil {
			fmt.Println("cups:", "warn: lpstat not found in PATH")
		}
	}

	osPrinters, err := ListOSPrinters()
	if err != nil {
		fmt.Println("os_printers:", "warn:", err.Error())
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
				fmt.Println("network:", "warn:", p.AgentIdentifier, "dial failed:", err.Error())
			} else {
				_ = conn.Close()
			}
		}
		if strings.TrimSpace(p.OSPrinterName) == "" && strings.TrimSpace(p.NetworkHost) == "" {
			fmt.Println("mapping:", "info:", p.AgentIdentifier, "will spool to disk (no os_printer_name/network_host)")
		}
	}

	if len(missingOS) > 0 {
		sort.Strings(missingOS)
		fmt.Println("mapping:", "warn: unknown os_printer_name values:")
		for _, m := range missingOS {
			fmt.Println("-", m)
		}
		fmt.Println("hint:", "Run: odoo-print-agent printers")
	}
}

func looksLikeHTML(b []byte) bool {
	s := strings.TrimSpace(strings.ToLower(string(b)))
	return strings.HasPrefix(s, "<!doctype html") || strings.HasPrefix(s, "<html")
}
