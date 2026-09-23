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

	ext, err := windowsPrintExt(job.JobType, payload)
	if err != nil {
		return err
	}

	sumatra, err := b.resolveSumatraPath()
	if err != nil {
		return err
	}

	tmpDir := os.TempDir()
	name := fmt.Sprintf("odoo_print_job_%d%s", job.ID, ext)
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

// windowsPrintExt chooses a file extension Sumatra can print.
// POS Print Master sends JPEG receipt images as job_type "raw".
func windowsPrintExt(jobType string, payload []byte) (string, error) {
	jt := strings.ToLower(strings.TrimSpace(jobType))
	switch {
	case jt == "pdf" || looksLikePDF(payload):
		return ".pdf", nil
	case looksLikeJPEG(payload):
		return ".jpg", nil
	case looksLikePNG(payload):
		return ".png", nil
	case jt == "raw" || jt == "escpos":
		return "", fmt.Errorf("windows os_printer_name cannot print ESC/POS binary; use network_host:9100 for thermal, or send PDF/image jobs")
	default:
		return "", fmt.Errorf("windows os_printer_name only supports pdf/jpeg/png jobs (got %q)", jobType)
	}
}

func looksLikePDF(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "%PDF"
}

func looksLikeJPEG(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff
}

func looksLikePNG(b []byte) bool {
	return len(b) >= 8 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4e && b[3] == 0x47
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
		candidate = filepath.Join(filepath.Dir(exe), "sumatra", "SumatraPDF.exe")
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
