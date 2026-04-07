package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SpoolBackend struct {
	baseDir string
}

func NewSpoolBackend(baseDir string) *SpoolBackend {
	return &SpoolBackend{baseDir: strings.TrimSpace(baseDir)}
}

func (b *SpoolBackend) Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error {
	_ = ctx

	dir := filepath.Join(b.baseDir, sanitizePathSegment(printer.AgentIdentifier))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	ext := ".bin"
	switch strings.ToLower(strings.TrimSpace(job.JobType)) {
	case "pdf":
		ext = ".pdf"
	case "escpos", "raw":
		ext = ".bin"
	}

	name := fmt.Sprintf("%s_job_%d%s", time.Now().UTC().Format("20060102_150405.000"), job.ID, ext)
	target := filepath.Join(dir, name)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func sanitizePathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}
