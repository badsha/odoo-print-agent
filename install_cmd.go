package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type installPaths struct {
	BinPath          string
	ConfigPath       string
	LogPath          string
	SpoolDir         string
	ServiceName      string
	RequiresRoot     bool
	LaunchdPlistPath string
	LaunchdDomain    string
	LaunchdTarget    string
	SystemdUnitPath  string
}

func installCmd(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:0", "UI listen address (use :0 for a random port)")
	noOpen := fs.Bool("no-open", false, "Do not open the browser automatically")
	requirePrinters := fs.Bool("require-printers", true, "Wait until at least one printer is configured")
	timeout := fs.Duration("timeout", 30*time.Minute, "Max time to wait for setup completion")
	_ = fs.Parse(args)

	euid := 0
	if runtime.GOOS != "windows" {
		euid = os.Geteuid()
	}
	paths, err := defaultInstallPaths(euid)
	if err != nil {
		logFatalf("install: %v", err)
	}

	if paths.RequiresRoot && euid != 0 {
		logFatalf("install must be run as root (use sudo)")
	}

	exe, err := os.Executable()
	if err != nil {
		logFatalf("executable: %v", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		logFatalf("executable: %v", err)
	}

	if err := ensureDir(filepath.Dir(paths.BinPath), 0o755); err != nil {
		logFatalf("install: %v", err)
	}
	if err := ensureDir(filepath.Dir(paths.ConfigPath), 0o755); err != nil {
		logFatalf("install: %v", err)
	}
	if err := ensureDir(filepath.Dir(paths.LogPath), 0o755); err != nil {
		logFatalf("install: %v", err)
	}
	if err := ensureDir(paths.SpoolDir, 0o755); err != nil {
		logFatalf("install: %v", err)
	}

	if err := copyFile(exe, paths.BinPath, 0o755); err != nil {
		logFatalf("install: %v", err)
	}

	if _, err := os.Stat(paths.ConfigPath); err != nil {
		if !os.IsNotExist(err) {
			logFatalf("install: stat config: %v", err)
		}
		cfg := DefaultConfig()
		cfg.SpoolDir = paths.SpoolDir
		cfg.LogFile = paths.LogPath
		cfg.LogLevel = "info"
		cfg.Printers = nil
		if err := cfg.Save(paths.ConfigPath); err != nil {
			logFatalf("install: write default config: %v", err)
		}
	}

	if err := installService(paths); err != nil {
		logFatalf("install: %v", err)
	}

	if strings.HasSuffix(*addr, ":0") {
		if picked, err := pickFreeAddr(*addr); err == nil {
			*addr = picked
		}
	}

	uiProc, uiURL, err := startUIProcess(paths.BinPath, paths.ConfigPath, *addr)
	if err != nil {
		logFatalf("install: start ui: %v", err)
	}
	defer func() {
		_ = stopProcess(uiProc)
	}()

	fmt.Println("Setup UI:", uiURL)
	if !*noOpen {
		_ = openBrowser(uiURL)
	}

	deadline := time.Now().Add(*timeout)
	for {
		if time.Now().After(deadline) {
			logFatalf("install: setup timed out (config: %s)", paths.ConfigPath)
		}
		cfg, err := LoadConfig(paths.ConfigPath)
		if err == nil {
			ready := strings.TrimSpace(cfg.OdooURL) != "" && strings.TrimSpace(cfg.APIKey) != ""
			if *requirePrinters {
				ready = ready && len(cfg.Printers) > 0
			}
			if ready {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	_ = stopProcess(uiProc)

	cfg, err := LoadConfig(paths.ConfigPath)
	if err != nil {
		logFatalf("install: reload config: %v", err)
	}
	initLogging(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for _, ln := range uiDoctorReport(ctx, cfg, paths.ConfigPath, 15*time.Second) {
		fmt.Println(ln)
	}

	if err := startService(paths); err != nil {
		logFatalf("install: start service: %v", err)
	}
	fmt.Println("installed and started service:", paths.ServiceName)
}

func defaultInstallPaths(euid int) (installPaths, error) {
	switch runtime.GOOS {
	case "darwin":
		if euid == 0 {
			cfgDir := "/Library/Application Support/OdooPrintAgent"
			logDir := "/Library/Logs/OdooPrintAgent"
			return installPaths{
				BinPath:          "/Applications/OdooPrintAgent/odoo-print-agent",
				ConfigPath:       filepath.Join(cfgDir, "config.json"),
				LogPath:          filepath.Join(logDir, "agent.jsonl"),
				SpoolDir:         filepath.Join(cfgDir, "spool"),
				ServiceName:      "com.odoo.printagent",
				RequiresRoot:     true,
				LaunchdPlistPath: "/Library/LaunchDaemons/com.odoo.printagent.plist",
				LaunchdDomain:    "system",
				LaunchdTarget:    "system/com.odoo.printagent",
			}, nil
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return installPaths{}, err
		}
		cfgDir := filepath.Join(home, "Library", "Application Support", "OdooPrintAgent")
		logDir := filepath.Join(home, "Library", "Logs", "OdooPrintAgent")
		uid := os.Getuid()
		return installPaths{
			BinPath:          filepath.Join(cfgDir, "odoo-print-agent"),
			ConfigPath:       filepath.Join(cfgDir, "config.json"),
			LogPath:          filepath.Join(logDir, "agent.jsonl"),
			SpoolDir:         filepath.Join(cfgDir, "spool"),
			ServiceName:      "com.odoo.printagent",
			RequiresRoot:     false,
			LaunchdPlistPath: filepath.Join(home, "Library", "LaunchAgents", "com.odoo.printagent.plist"),
			LaunchdDomain:    fmt.Sprintf("gui/%d", uid),
			LaunchdTarget:    fmt.Sprintf("gui/%d/com.odoo.printagent", uid),
		}, nil
	case "linux":
		return installPaths{
			BinPath:         "/opt/odoo-print-agent/odoo-print-agent",
			ConfigPath:      "/etc/odoo-print-agent/config.json",
			LogPath:         "/var/log/odoo-print-agent/agent.jsonl",
			SpoolDir:        "/var/lib/odoo-print-agent/spool",
			ServiceName:     "odoo-print-agent",
			RequiresRoot:    true,
			SystemdUnitPath: "/etc/systemd/system/odoo-print-agent.service",
		}, nil
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if strings.TrimSpace(programFiles) == "" {
			programFiles = `C:\Program Files`
		}
		programData := os.Getenv("ProgramData")
		if strings.TrimSpace(programData) == "" {
			programData = `C:\ProgramData`
		}
		cfgDir := filepath.Join(programData, "OdooPrintAgent")
		logDir := filepath.Join(cfgDir, "logs")
		return installPaths{
			BinPath:      filepath.Join(programFiles, "OdooPrintAgent", "odoo-print-agent.exe"),
			ConfigPath:   filepath.Join(cfgDir, "config.json"),
			LogPath:      filepath.Join(logDir, "agent.jsonl"),
			SpoolDir:     filepath.Join(cfgDir, "spool"),
			ServiceName:  "OdooPrintAgent",
			RequiresRoot: false,
		}, nil
	default:
		return installPaths{}, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installService(p installPaths) error {
	switch runtime.GOOS {
	case "darwin":
		plist := launchdPlist(p.ServiceName, p.BinPath, p.ConfigPath, p.LogPath)
		if err := ensureDir(filepath.Dir(p.LaunchdPlistPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p.LaunchdPlistPath, []byte(plist), 0o644); err != nil {
			return fmt.Errorf("write plist: %w", err)
		}
		return nil
	case "linux":
		unit := systemdUnit(p.BinPath, p.ConfigPath, p.LogPath)
		if err := os.WriteFile(p.SystemdUnitPath, []byte(unit), 0o644); err != nil {
			return fmt.Errorf("write unit: %w", err)
		}
		return nil
	case "windows":
		return nil
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func startService(p installPaths) error {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("launchctl", "bootout", p.LaunchdDomain, p.LaunchdPlistPath).Run()
		if err := exec.Command("launchctl", "bootstrap", p.LaunchdDomain, p.LaunchdPlistPath).Run(); err != nil {
			if p.LaunchdDomain == "system" {
				_ = exec.Command("launchctl", "load", p.LaunchdPlistPath).Run()
			} else {
				return err
			}
		}
		_ = exec.Command("launchctl", "enable", p.LaunchdTarget).Run()
		_ = exec.Command("launchctl", "kickstart", "-k", p.LaunchdTarget).Run()
		return nil
	case "linux":
		_ = exec.Command("systemctl", "daemon-reload").Run()
		if err := exec.Command("systemctl", "enable", "--now", p.ServiceName).Run(); err != nil {
			return err
		}
		return nil
	case "windows":
		return windowsEnsureService(p.ServiceName, p.BinPath, p.ConfigPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func pickFreeAddr(addr string) (string, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return "", err
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().String(), nil
}

func startUIProcess(binPath string, configPath string, addr string) (*exec.Cmd, string, error) {
	cmd := exec.Command(binPath, "ui", "--config", configPath, "--addr", addr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, "", err
	}
	uiURL := "http://" + addr + "/"
	return cmd, uiURL, nil
}

func stopProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Signal(os.Interrupt)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	case <-done:
	}
	return nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		user := strings.TrimSpace(os.Getenv("SUDO_USER"))
		if user != "" && user != "root" {
			return exec.Command("/usr/bin/su", "-l", user, "-c", "/usr/bin/open "+shellQuote(url)).Run()
		}
		return exec.Command("/usr/bin/open", url).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func shellQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func ensureDir(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	return nil
}

func copyFile(src string, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func launchdPlist(label string, binPath string, configPath string, logPath string) string {
	escapedLog := xmlEscape(logPath)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
      <string>%s</string>
      <string>run</string>
      <string>--config</string>
      <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
  </dict>
</plist>
`, xmlEscape(label), xmlEscape(binPath), xmlEscape(configPath), escapedLog, escapedLog)
}

func systemdUnit(binPath string, configPath string, logPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Odoo Print Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s run --config %s
Restart=always
RestartSec=2
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=multi-user.target
`, binPath, configPath, logPath, logPath)
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func windowsEnsureService(serviceName string, binPath string, configPath string) error {
	binPath = strings.ReplaceAll(binPath, `"`, "")
	configPath = strings.ReplaceAll(configPath, `"`, "")
	full := fmt.Sprintf(`"%s" run --config "%s"`, binPath, configPath)

	_ = exec.Command("sc.exe", "stop", serviceName).Run()
	if err := exec.Command("sc.exe", "query", serviceName).Run(); err == nil {
		_ = exec.Command("sc.exe", "config", serviceName, "start=", "auto", "binPath=", full).Run()
	} else {
		if err := exec.Command("sc.exe", "create", serviceName, "start=", "auto", "binPath=", full, "DisplayName=", "Odoo Print Agent").Run(); err != nil {
			return err
		}
	}
	_ = exec.Command("sc.exe", "description", serviceName, "Odoo Print Agent (LL Print Platform)").Run()
	if err := exec.Command("sc.exe", "start", serviceName).Run(); err != nil {
		return err
	}
	return nil
}
