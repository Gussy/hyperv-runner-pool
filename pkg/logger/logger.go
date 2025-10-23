package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Setup creates and configures the application logger
func Setup(logLevel, logFormat, logDirectory string) *slog.Logger {
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

	// Create multi-writer for both stdout and file
	var writers []io.Writer
	writers = append(writers, os.Stdout)

	// Add file writer if log directory is specified
	if logDirectory != "" {
		logFile, err := openDailyLogFile(logDirectory)
		if err != nil {
			// Log error to stdout but continue with stdout-only logging
			fmt.Fprintf(os.Stderr, "Warning: Failed to open log file: %v\n", err)
		} else {
			writers = append(writers, logFile)
		}
	}

	multiWriter := io.MultiWriter(writers...)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if logFormat == "json" {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	return slog.New(handler)
}

// openDailyLogFile creates or opens the log file for the current date
func openDailyLogFile(logDirectory string) (*os.File, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDirectory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Generate filename based on current date (YYYY-MM-DD.log)
	filename := time.Now().Format("2006-01-02") + ".log"
	logPath := filepath.Join(logDirectory, filename)

	// Open file in append mode, create if doesn't exist
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %w", logPath, err)
	}

	return file, nil
}
