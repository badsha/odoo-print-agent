package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func ListOSPrinters() ([]string, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
		out, err := exec.Command("lpstat", "-p").CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("lpstat failed: %v: %s", err, strings.TrimSpace(string(out)))
		}
		lines := strings.Split(string(out), "\n")
		var printers []string
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if !strings.HasPrefix(ln, "printer ") {
				continue
			}
			parts := strings.Fields(ln)
			if len(parts) < 2 {
				continue
			}
			printers = append(printers, parts[1])
		}
		return printers, nil
	case "windows":
		cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-Printer | Select-Object -ExpandProperty Name")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("powershell Get-Printer failed: %v: %s", err, strings.TrimSpace(string(out)))
		}
		lines := strings.Split(string(out), "\n")
		var printers []string
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			printers = append(printers, ln)
		}
		return printers, nil
	default:
		return nil, fmt.Errorf("unsupported os: %s", runtime.GOOS)
	}
}
