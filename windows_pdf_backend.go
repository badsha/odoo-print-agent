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

type WindowsPDFBackend struct {
	sumatraPath string
}

func NewWindowsPDFBackend(sumatraPath string) *WindowsPDFBackend {
	if runtime.GOOS != "windows" {
		return nil
	}
	return &WindowsPDFBackend{sumatraPath: strings.TrimSpace(sumatraPath)}
}

func (b *WindowsPDFBackend) Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("windows pdf backend is only supported on windows")
	}
	prn := strings.TrimSpace(printer.OSPrinterName)
	if prn == "" {
		return fmt.Errorf("missing os_printer_name")
	}
	if strings.ToLower(strings.TrimSpace(job.JobType)) != "pdf" {
		return fmt.Errorf("windows pdf backend only supports pdf jobs (got %q)", job.JobType)
	}

	sumatra, err := b.resolveSumatraPath()
	if err != nil {
		return err
	}

	tmpDir := os.TempDir()
	name := fmt.Sprintf("odoo_print_job_%d.pdf", job.ID)
	target := filepath.Join(tmpDir, name)
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		return err
	}
	defer os.Remove(target)

	args := []string{"-print-to", prn, "-silent", target}
	cmd := exec.CommandContext(ctx, sumatra, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sumatra failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (b *WindowsPDFBackend) resolveSumatraPath() (string, error) {
	if strings.TrimSpace(b.sumatraPath) != "" {
		if _, err := os.Stat(b.sumatraPath); err == nil {
			return b.sumatraPath, nil
		}
	}
	if p, err := exec.LookPath("SumatraPDF.exe"); err == nil {
		return p, nil
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "SumatraPDF.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	for _, candidate := range []string{
		`C:\Program Files\SumatraPDF\SumatraPDF.exe`,
		`C:\Program Files (x86)\SumatraPDF\SumatraPDF.exe`,
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("SumatraPDF.exe not found (set sumatra_pdf_path in config or install SumatraPDF)")
}
