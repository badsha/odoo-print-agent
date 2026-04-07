package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cleanupStartup(cfg *Config) {
	cleanupTempFiles(48 * time.Hour)
	if cfg != nil && strings.TrimSpace(cfg.SpoolDir) != "" {
		cleanupSpoolTmp(cfg.SpoolDir, 48*time.Hour)
	}
}

func cleanupTempFiles(olderThan time.Duration) {
	dir := os.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "odoo_print_job_") && !strings.HasPrefix(name, "odoo_print_agent_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}

func cleanupSpoolTmp(spoolDir string, olderThan time.Duration) {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-olderThan)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".tmp") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(spoolDir, name))
	}
}
