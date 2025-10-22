package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Setup creates and configures the application logger
func Setup(logLevel, logFormat string) *slog.Logger {
	// Parse log level
	logLevel = strings.ToLower(logLevel)
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Parse log format
	logFormat = strings.ToLower(logFormat)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
