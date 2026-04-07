package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func setupCmd(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	odooURL := fs.String("odoo-url", "", "Odoo base URL")
	apiKey := fs.String("api-key", "", "Agent API key")
	osPrinterName := fs.String("os-printer-name", "", "Set os_printer_name for all configured printers")
	spoolDir := fs.String("spool-dir", "", "Override spool_dir")
	sumatraPath := fs.String("sumatra-pdf-path", "", "Override sumatra_pdf_path (Windows PDF printing)")
	logFile := fs.String("log-file", "", "Write JSON logs to this file (rotated)")
	logLevel := fs.String("log-level", "", "Log level: debug|info|warn|error")
	runTest := fs.Bool("test-print", false, "Run test print after saving config")
	timeout := fs.Duration("timeout", 15*time.Second, "Doctor timeout")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	cfg, err := LoadConfig(absPath)
	if err != nil {
		logFatalf("load config: %v", err)
	}
	initLogging(cfg)

	in := bufio.NewReader(os.Stdin)
	fmt.Println("config:", absPath)

	if strings.TrimSpace(*odooURL) != "" {
		cfg.OdooURL = strings.TrimSpace(*odooURL)
	} else {
		cfg.OdooURL = promptLine(in, "Odoo URL", cfg.OdooURL)
	}
	if strings.TrimSpace(*apiKey) != "" {
		cfg.APIKey = strings.TrimSpace(*apiKey)
	} else {
		cfg.APIKey = promptLine(in, "API Key", cfg.APIKey)
	}

	if strings.TrimSpace(*spoolDir) != "" {
		cfg.SpoolDir = strings.TrimSpace(*spoolDir)
	}
	if strings.TrimSpace(*sumatraPath) != "" {
		cfg.SumatraPDFPath = strings.TrimSpace(*sumatraPath)
	}
	if strings.TrimSpace(*logFile) != "" {
		cfg.LogFile = strings.TrimSpace(*logFile)
	}
	if strings.TrimSpace(*logLevel) != "" {
		cfg.LogLevel = strings.TrimSpace(*logLevel)
	}

	osPrinters, err := ListOSPrinters()
	if err != nil {
		fmt.Println("warn:", "list printers:", err.Error())
		osPrinters = nil
	}

	if strings.TrimSpace(*osPrinterName) != "" {
		for i := range cfg.Printers {
			cfg.Printers[i].OSPrinterName = strings.TrimSpace(*osPrinterName)
			cfg.Printers[i].NetworkHost = ""
			cfg.Printers[i].NetworkPort = 0
		}
	} else {
		requireSelection := len(cfg.Printers) == 0
		selected := promptPrinterSelection(in, osPrinters, requireSelection)
		if len(selected) > 0 {
			cfg.Printers = buildPrinterConfigs(selected)
		} else {
			cfg.Printers = promptPrinterMappings(in, cfg.Printers, osPrinters)
		}
	}

	if err := cfg.Save(absPath); err != nil {
		logFatalf("save config: %v", err)
	}
	fmt.Println("saved:", absPath)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	for _, ln := range uiDoctorReport(ctx, cfg, absPath, *timeout) {
		fmt.Println(ln)
	}

	if *runTest {
		if err := setupRunTestPrint(cfg, absPath, nil); err != nil {
			logFatalf("test print: %v", err)
		}
		return
	}

	yn := strings.ToLower(strings.TrimSpace(promptLine(in, "Run test print now? (y/N)", "")))
	if yn == "y" || yn == "yes" {
		if err := setupRunTestPrint(cfg, absPath, in); err != nil {
			logFatalf("test print: %v", err)
		}
	}
}

func setupRunTestPrint(cfg *Config, configPath string, in *bufio.Reader) error {
	if len(cfg.Printers) == 0 {
		return fmt.Errorf("no printers configured")
	}

	p, err := selectTestPrinter(cfg.Printers, in)
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.OSPrinterName) == "" && strings.TrimSpace(p.NetworkHost) == "" {
		return fmt.Errorf("printer %q has no mapping (os_printer_name or network_host)", p.AgentIdentifier)
	}
	if strings.TrimSpace(p.OSPrinterName) == "" {
		return fmt.Errorf("test print currently requires os_printer_name mapping")
	}

	backend := NewRoutingBackend(cfg)
	return runTestPrint(backend, p)
}

func selectTestPrinter(printers []PrinterConfig, in *bufio.Reader) (PrinterConfig, error) {
	if len(printers) == 0 {
		return PrinterConfig{}, fmt.Errorf("no printers configured")
	}
	if in == nil {
		for _, p := range printers {
			if strings.TrimSpace(p.OSPrinterName) != "" || strings.TrimSpace(p.NetworkHost) != "" {
				return p, nil
			}
		}
		return PrinterConfig{}, fmt.Errorf("no mapped printers available for test print")
	}

	if len(printers) == 1 {
		return printers[0], nil
	}
	for i, p := range printers {
		fmt.Printf("%d) %s (%s)\n", i+1, p.Name, p.AgentIdentifier)
	}
	raw := promptLine(in, "Select printer number", "1")
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 1 || n > len(printers) {
		return PrinterConfig{}, fmt.Errorf("invalid selection")
	}
	return printers[n-1], nil
}

func promptLine(in *bufio.Reader, label string, current string) string {
	cur := strings.TrimSpace(current)
	if cur != "" {
		fmt.Printf("%s [%s]: ", label, cur)
	} else {
		fmt.Printf("%s: ", label)
	}
	s, _ := in.ReadString('\n')
	s = strings.TrimSpace(s)
	if s == "" {
		return cur
	}
	return s
}

func promptPrinterSelection(in *bufio.Reader, osPrinters []string, requireSelection bool) []string {
	if len(osPrinters) == 0 {
		return nil
	}
	for {
		fmt.Println("Detected OS printers:")
		for i, p := range osPrinters {
			fmt.Printf("%d) %s\n", i+1, p)
		}
		if requireSelection {
			fmt.Println("Select printers to expose to Odoo (comma-separated numbers).")
			fmt.Println("Tip: enter 0 to skip and keep printers empty.")
		} else {
			fmt.Println("Select printers to expose to Odoo (comma-separated numbers). Leave empty to keep current printers.")
		}
		fmt.Print("Selection: ")
		raw, _ := in.ReadString('\n')
		raw = strings.TrimSpace(raw)
		if raw == "" {
			if requireSelection {
				fmt.Println("warn:", "please select at least one printer (or enter 0 to skip)")
				continue
			}
			return nil
		}
		if raw == "0" {
			return nil
		}
		idxs, err := parseSelection(raw, len(osPrinters))
		if err != nil {
			fmt.Println("warn:", err.Error())
			if requireSelection {
				continue
			}
			return nil
		}
		var selected []string
		for _, i := range idxs {
			selected = append(selected, osPrinters[i-1])
		}
		return selected
	}
}

func promptPrinterMappings(in *bufio.Reader, printers []PrinterConfig, osPrinters []string) []PrinterConfig {
	if len(printers) == 0 {
		return printers
	}
	if len(osPrinters) == 0 {
		return printers
	}

	fmt.Println("Map configured printers to OS printer queues.")
	for i := range printers {
		p := printers[i]
		fmt.Printf("%s (%s)\n", p.Name, p.AgentIdentifier)
		for j, op := range osPrinters {
			fmt.Printf("%d) %s\n", j+1, op)
		}
		raw := promptLine(in, "Select OS printer number (empty to keep)", "")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > len(osPrinters) {
			fmt.Println("warn:", "invalid selection; skipping")
			continue
		}
		printers[i].OSPrinterName = osPrinters[n-1]
		printers[i].NetworkHost = ""
		printers[i].NetworkPort = 0
	}
	return printers
}

func parseSelection(raw string, max int) ([]int, error) {
	fields := strings.Split(raw, ",")
	seen := make(map[int]struct{}, len(fields))
	var out []int
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q", f)
		}
		if n < 1 || n > max {
			return nil, fmt.Errorf("selection %d out of range", n)
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	sort.Ints(out)
	return out, nil
}

func buildPrinterConfigs(osPrinters []string) []PrinterConfig {
	var printers []PrinterConfig
	for _, p := range osPrinters {
		identifier := slugifyIdentifier(p)
		printers = append(printers, PrinterConfig{
			AgentIdentifier: identifier,
			Name:            p,
			PrinterType:     "report",
			Code:            makeCode(identifier),
			OSPrinterName:   p,
		})
	}
	return printers
}

func slugifyIdentifier(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "printer"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func makeCode(identifier string) string {
	identifier = strings.ToUpper(identifier)
	identifier = strings.ReplaceAll(identifier, "_", "")
	if identifier == "" {
		return "PRN"
	}
	if len(identifier) > 8 {
		return identifier[:8]
	}
	return identifier
}
