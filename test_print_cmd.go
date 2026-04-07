package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"
)

func testPrintCmd(args []string) {
	fs := flag.NewFlagSet("test-print", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to config.json (defaults to ./config.json)")
	printerID := fs.String("printer", "", "Printer agent_identifier to test")
	timeout := fs.Duration("timeout", 15*time.Second, "Print timeout")
	_ = fs.Parse(args)

	absPath := resolveConfigPath(*configPath)
	cfg, err := LoadConfig(absPath)
	if err != nil {
		logFatalf("load config: %v", err)
	}
	initLogging(cfg)

	var p PrinterConfig
	switch {
	case strings.TrimSpace(*printerID) != "":
		found := false
		for _, pcfg := range cfg.Printers {
			if strings.TrimSpace(pcfg.AgentIdentifier) == strings.TrimSpace(*printerID) {
				p = pcfg
				found = true
				break
			}
		}
		if !found {
			logFatalf("unknown printer %q (use: odoo-print-agent printers / setup)", strings.TrimSpace(*printerID))
		}
	case len(cfg.Printers) == 1:
		p = cfg.Printers[0]
	default:
		logFatalf("missing --printer (multiple printers configured)")
	}

	if strings.TrimSpace(p.OSPrinterName) == "" {
		logFatalf("printer %q has no os_printer_name mapping", p.AgentIdentifier)
	}

	backend := NewRoutingBackend(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	start := time.Now()
	if err := runTestPrintWithContext(ctx, backend, p); err != nil {
		logFatalf("print failed: %v", err)
	}
	fmt.Printf("ok: printed test page to %s (%s) in %s\n", p.Name, p.AgentIdentifier, time.Since(start).Truncate(time.Millisecond))
}

func runTestPrint(backend PrintBackend, printer PrinterConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return runTestPrintWithContext(ctx, backend, printer)
}

func runTestPrintWithContext(ctx context.Context, backend PrintBackend, printer PrinterConfig) error {
	job := Job{
		ID:      time.Now().Unix(),
		Name:    "Test Print",
		JobType: "pdf",
	}
	return backend.Print(ctx, printer, job, testPDFBytes)
}

var testPDFBytes = []byte("%PDF-1.4\n" +
	"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
	"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
	"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n" +
	"4 0 obj\n<< /Length 86 >>\nstream\n" +
	"BT\n/F1 24 Tf\n72 720 Td\n(Odoo Print Agent Test Page) Tj\n0 -36 Td\n(If you can read this, printing works.) Tj\nET\n" +
	"\nendstream\nendobj\n" +
	"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
	"xref\n0 6\n0000000000 65535 f \n" +
	"0000000009 00000 n \n" +
	"0000000058 00000 n \n" +
	"0000000115 00000 n \n" +
	"0000000250 00000 n \n" +
	"0000000388 00000 n \n" +
	"trailer\n<< /Size 6 /Root 1 0 R >>\n" +
	"startxref\n468\n%%EOF\n")
