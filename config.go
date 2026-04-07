package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	OdooURL             string          `json:"odoo_url"`
	APIKey              string          `json:"api_key"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	LeaseSeconds        int             `json:"lease_seconds"`
	Limit               int             `json:"limit"`
	SpoolDir            string          `json:"spool_dir"`
	SumatraPDFPath      string          `json:"sumatra_pdf_path"`
	LogFile             string          `json:"log_file"`
	LogLevel            string          `json:"log_level"`
	Printers            []PrinterConfig `json:"printers"`
}

type PrinterConfig struct {
	AgentIdentifier string `json:"agent_identifier"`
	Name            string `json:"name"`
	PrinterType     string `json:"printer_type"`
	Code            string `json:"code"`
	OSPrinterName   string `json:"os_printer_name"`
	NetworkHost     string `json:"network_host"`
	NetworkPort     int    `json:"network_port"`
}

func DefaultConfig() *Config {
	return &Config{
		OdooURL:             "",
		APIKey:              "",
		PollIntervalSeconds: 3,
		LeaseSeconds:        30,
		Limit:               20,
		SpoolDir:            "spool",
		Printers: []PrinterConfig{
			{
				AgentIdentifier: "test_printer_1",
				Name:            "Test Printer",
				PrinterType:     "receipt",
				Code:            "TEST1",
			},
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if err := cfg.Normalize(filepath.Dir(path)); err != nil {
				return nil, fmt.Errorf("normalize default config: %w", err)
			}
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Normalize(filepath.Dir(path)); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) Normalize(baseDir string) error {
	c.OdooURL = strings.TrimSpace(c.OdooURL)
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.SumatraPDFPath = strings.TrimSpace(c.SumatraPDFPath)
	c.LogFile = strings.TrimSpace(c.LogFile)
	c.LogLevel = strings.TrimSpace(c.LogLevel)
	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = 3
	}
	if c.LeaseSeconds <= 0 {
		c.LeaseSeconds = 30
	}
	if c.Limit <= 0 {
		c.Limit = 20
	}
	if strings.TrimSpace(c.SpoolDir) == "" {
		c.SpoolDir = "spool"
	}
	if !filepath.IsAbs(c.SpoolDir) {
		c.SpoolDir = filepath.Join(baseDir, c.SpoolDir)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFile != "" && !filepath.IsAbs(c.LogFile) {
		c.LogFile = filepath.Join(baseDir, c.LogFile)
	}
	for i := range c.Printers {
		c.Printers[i].AgentIdentifier = strings.TrimSpace(c.Printers[i].AgentIdentifier)
		c.Printers[i].Name = strings.TrimSpace(c.Printers[i].Name)
		c.Printers[i].PrinterType = strings.TrimSpace(c.Printers[i].PrinterType)
		c.Printers[i].Code = strings.TrimSpace(c.Printers[i].Code)
		c.Printers[i].OSPrinterName = strings.TrimSpace(c.Printers[i].OSPrinterName)
		c.Printers[i].NetworkHost = strings.TrimSpace(c.Printers[i].NetworkHost)
		if c.Printers[i].NetworkHost != "" && c.Printers[i].NetworkPort <= 0 {
			c.Printers[i].NetworkPort = 9100
		}
	}
	return nil
}

func (c *Config) Save(path string) error {
	baseDir := filepath.Dir(path)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}

	if err := c.Normalize(baseDir); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
