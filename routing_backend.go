package main

import (
	"context"
	"fmt"
	"strings"
)

type RoutingBackend struct {
	cups  *CUPSBackend
	raw   *RawTCPBackend
	spool *SpoolBackend
}

func NewRoutingBackend(spoolDir string) *RoutingBackend {
	return &RoutingBackend{
		cups:  &CUPSBackend{},
		raw:   &RawTCPBackend{},
		spool: NewSpoolBackend(spoolDir),
	}
}

func (b *RoutingBackend) Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error {
	if strings.TrimSpace(printer.NetworkHost) != "" {
		if !isRawJobType(job.JobType) {
			return fmt.Errorf("network printing only supports raw/escpos jobs (got %q)", job.JobType)
		}
		return b.raw.Print(ctx, printer, job, payload)
	}
	if strings.TrimSpace(printer.OSPrinterName) != "" {
		return b.cups.Print(ctx, printer, job, payload)
	}
	return b.spool.Print(ctx, printer, job, payload)
}

func isRawJobType(jobType string) bool {
	switch strings.ToLower(strings.TrimSpace(jobType)) {
	case "raw", "escpos":
		return true
	default:
		return false
	}
}
