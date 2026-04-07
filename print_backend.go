package main

import "context"

type PrintBackend interface {
	Print(ctx context.Context, printer PrinterConfig, job Job, payload []byte) error
}
