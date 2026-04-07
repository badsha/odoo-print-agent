package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
)

var (
	currentLogLevel = levelInfo
	logMu           sync.Mutex
)

type logEntry struct {
	TS      string         `json:"ts"`
	Level   string         `json:"level"`
	Event   string         `json:"event,omitempty"`
	Message string         `json:"message,omitempty"`
	Fields  map[string]any `json:"fields,omitempty"`
}

func initLogging(cfg *Config) {
	if cfg == nil {
		return
	}
	currentLogLevel = parseLogLevel(cfg.LogLevel)

	var out io.Writer = os.Stdout
	if strings.TrimSpace(cfg.LogFile) != "" {
		_ = os.MkdirAll(filepathDir(cfg.LogFile), 0o755)
		out = io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    10,
			MaxBackups: 5,
			Compress:   false,
		})
	} else if runtime.GOOS == "windows" {
		out = os.Stdout
	}

	log.SetFlags(0)
	log.SetOutput(out)
}

func logDebug(event string, message string, fields map[string]any) {
	logWithLevel(levelDebug, "debug", event, message, fields)
}

func logInfo(event string, message string, fields map[string]any) {
	logWithLevel(levelInfo, "info", event, message, fields)
}

func logWarn(event string, message string, fields map[string]any) {
	logWithLevel(levelWarn, "warn", event, message, fields)
}

func logError(event string, message string, fields map[string]any) {
	logWithLevel(levelError, "error", event, message, fields)
}

func logWithLevel(lvl logLevel, lvlStr, event, message string, fields map[string]any) {
	if lvl < currentLogLevel {
		return
	}
	e := logEntry{
		TS:      time.Now().UTC().Format(time.RFC3339Nano),
		Level:   lvlStr,
		Event:   strings.TrimSpace(event),
		Message: strings.TrimSpace(message),
	}
	if len(fields) > 0 {
		e.Fields = fields
	}
	b, err := json.Marshal(e)
	if err != nil {
		logMu.Lock()
		log.Printf(`{"ts":%q,"level":"error","event":"log_marshal_failed","message":%q}`, time.Now().UTC().Format(time.RFC3339Nano), err.Error())
		logMu.Unlock()
		return
	}
	logMu.Lock()
	log.Print(string(b))
	logMu.Unlock()
}

func parseLogLevel(s string) logLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return levelDebug
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

func filepathDir(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i <= 0 {
		return "."
	}
	return p[:i]
}

func logFatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logError("fatal", msg, nil)
	os.Exit(1)
}
