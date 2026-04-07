package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type CUPSBackend struct{}

func (b *CUPSBackend) Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("os_printer_name printing is not supported on windows in this agent build")
	}
	prn := strings.TrimSpace(printer.OSPrinterName)
	if prn == "" {
		return fmt.Errorf("missing os_printer_name")
	}

	ext := ".bin"
	switch strings.ToLower(strings.TrimSpace(job.JobType)) {
	case "pdf":
		ext = ".pdf"
	case "escpos", "raw":
		ext = ".bin"
	default:
		ext = ".bin"
	}

	tmpDir := os.TempDir()
	name := fmt.Sprintf("odoo_print_job_%d%s", job.ID, ext)
	target := filepath.Join(tmpDir, name)
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		return err
	}
	defer os.Remove(target)

	args := []string{"-d", prn, "-t", safeTitle(job.Name), target}
	if isRawJobType(job.JobType) {
		args = []string{"-d", prn, "-o", "raw", "-t", safeTitle(job.Name), target}
	}

	cmd := exec.CommandContext(ctx, "lp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lp failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func safeTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Odoo Print Job"
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
