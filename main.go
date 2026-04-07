package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	args := os.Args[1:]
	cmd := "run"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}

	switch cmd {
	case "run":
		runCmd(args)
	case "configure":
		configureCmd(args)
	case "doctor":
		doctorCmd(args)
	case "printers":
		printersCmd(args)
	case "ui":
		uiCmd(args)
	case "version":
		fmt.Printf("odoo-print-agent %s\n", versionString())
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(2)
	}
}

func versionString() string {
	s := Version
	if strings.TrimSpace(Commit) != "" {
		s += "+" + strings.TrimSpace(Commit)
	}
	if strings.TrimSpace(BuildDate) != "" {
		s += " (" + strings.TrimSpace(BuildDate) + ")"
	}
	return s
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	once := fs.Bool("once", false, "Run one sync+poll cycle and exit")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	cfg, err := LoadConfig(absPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if strings.TrimSpace(cfg.OdooURL) == "" {
		log.Fatalf("missing odoo_url in config: %s", absPath)
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		log.Fatalf("missing api_key in config: %s", absPath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registerSignals(cancel)

	client := NewAPIClient(cfg.OdooURL, cfg.APIKey)
	backend := NewRoutingBackend(cfg.SpoolDir)

	if *once {
		if err := RunOnce(ctx, cfg, client, backend); err != nil {
			log.Fatalf("run once: %v", err)
		}
		return
	}

	ticker := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		start := time.Now()
		if err := RunOnce(ctx, cfg, client, backend); err != nil {
			log.Printf("cycle error: %v", err)
		}
		elapsed := time.Since(start)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if elapsed > 0 {
				continue
			}
		}
	}
}

func configureCmd(args []string) {
	fs := flag.NewFlagSet("configure", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	odooURL := fs.String("odoo-url", "", "Odoo base URL")
	odooURLAlt := fs.String("odoo_url", "", "Odoo base URL")
	apiKey := fs.String("api-key", "", "Agent API key")
	skipValidate := fs.Bool("skip-validate", false, "Skip connectivity validation")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	cfg, err := LoadConfig(absPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(*odooURL) != "" {
		cfg.OdooURL = strings.TrimSpace(*odooURL)
	} else if strings.TrimSpace(*odooURLAlt) != "" {
		cfg.OdooURL = strings.TrimSpace(*odooURLAlt)
	}
	if strings.TrimSpace(*apiKey) != "" {
		cfg.APIKey = strings.TrimSpace(*apiKey)
	}
	if strings.TrimSpace(cfg.OdooURL) == "" {
		log.Fatalf("missing odoo_url")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		log.Fatalf("missing api_key")
	}

	if !*skipValidate {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		client := NewAPIClient(cfg.OdooURL, cfg.APIKey)
		if _, err := client.GetJobs(ctx, 5, 1); err != nil {
			log.Fatalf("validate failed: %v", err)
		}
	}
	if err := cfg.Save(absPath); err != nil {
		log.Fatalf("save config: %v", err)
	}
	fmt.Printf("saved %s\n", absPath)
}

func resolveConfigPath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		if _, err := os.Stat("config.json"); err == nil {
			p = "config.json"
		} else {
			userCfgDir, err := os.UserConfigDir()
			if err != nil || strings.TrimSpace(userCfgDir) == "" {
				p = "config.json"
			} else {
				p = filepath.Join(userCfgDir, "odoo-print-agent", "config.json")
			}
		}
	}
	absPath, err := filepath.Abs(p)
	if err != nil {
		log.Fatalf("invalid config path: %v", err)
	}
	return absPath
}

func printersCmd(args []string) {
	fs := flag.NewFlagSet("printers", flag.ExitOnError)
	_ = fs.Parse(args)

	printers, err := ListOSPrinters()
	if err != nil {
		log.Fatalf("list printers: %v", err)
	}
	for _, p := range printers {
		fmt.Println(p)
	}
}

func registerSignals(cancel func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
}

func RunOnce(ctx context.Context, cfg *Config, client *APIClient, backend PrintBackend) error {
	log.Printf("sync printers: %d configured", len(cfg.Printers))
	if err := client.SyncPrinters(ctx, cfg.Printers); err != nil {
		return fmt.Errorf("sync printers: %w", err)
	}
	log.Printf("sync printers: ok")

	jobs, err := client.GetJobs(ctx, cfg.LeaseSeconds, cfg.Limit)
	if err != nil {
		return fmt.Errorf("get jobs: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}

	printerByID := make(map[string]PrinterConfig, len(cfg.Printers))
	for _, p := range cfg.Printers {
		printerByID[p.AgentIdentifier] = p
	}

	for _, j := range jobs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := handleJob(ctx, client, backend, printerByID, j); err != nil {
			log.Printf("job %d error: %v", j.ID, err)
		}
	}
	return nil
}

func handleJob(ctx context.Context, client *APIClient, backend PrintBackend, printerByID map[string]PrinterConfig, job Job) error {
	pcfg, ok := printerByID[job.PrinterIdentifier]
	if !ok {
		_ = client.FailJob(ctx, job.ID, job.LeaseUUID, fmt.Sprintf("unknown printer_identifier %q", job.PrinterIdentifier))
		return fmt.Errorf("unknown printer_identifier %q", job.PrinterIdentifier)
	}

	if err := client.AckJob(ctx, job.ID, job.LeaseUUID); err != nil {
		return fmt.Errorf("ack: %w", err)
	}

	payload, err := job.DecodePayload()
	if err != nil {
		_ = client.FailJob(ctx, job.ID, job.LeaseUUID, fmt.Sprintf("invalid payload: %v", err))
		return fmt.Errorf("decode payload: %w", err)
	}

	if err := backend.Print(ctx, pcfg, job, payload); err != nil {
		_ = client.FailJob(ctx, job.ID, job.LeaseUUID, err.Error())
		return fmt.Errorf("print: %w", err)
	}

	if err := client.DoneJob(ctx, job.ID, job.LeaseUUID); err != nil {
		return fmt.Errorf("done: %w", err)
	}
	return nil
}
