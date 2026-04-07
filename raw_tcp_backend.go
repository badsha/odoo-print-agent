package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type RawTCPBackend struct{}

func (b *RawTCPBackend) Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error {
	_ = job

	host := strings.TrimSpace(printer.NetworkHost)
	if host == "" {
		return fmt.Errorf("missing network_host")
	}
	port := printer.NetworkPort
	if port <= 0 {
		port = 9100
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	_ = conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	n, err := conn.Write(payload)
	if err != nil {
		return err
	}
	if n != len(payload) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(payload))
	}
	return nil
}
