package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Logger adapts slog.Logger to implement ports.Logger interface
type Logger struct {
	logger *slog.Logger
}

// Config holds logger configuration
type Config struct {
	Level             string
	EnableConsole     bool
	EnableFile        bool
	MaxFileSize       string
	FileRetentionDays int
}

// New creates a new Logger with the given configuration
func New(cfg Config) (*Logger, error) {
	// Parse log level
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var writers []io.Writer

	// Console output
	if cfg.EnableConsole {
		writers = append(writers, os.Stdout)
	}

	// File output (simple daily rotation)
	if cfg.EnableFile {
		if err := os.MkdirAll("./logs", 0755); err != nil {
			return nil, fmt.Errorf("failed to create logs directory: %w", err)
		}

		// Create log file with date in filename for daily rotation
		logFilename := filepath.Join("./logs", fmt.Sprintf("app-%s.log", time.Now().Format("2006-01-02")))
		logFile, err := os.OpenFile(logFilename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		writers = append(writers, logFile)
	}

	// Default to stdout if no writers configured
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	output := io.MultiWriter(writers...)

	// Create slog handler with text output for readability
	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)

	return &Logger{
		logger: logger,
	}, nil
}

// Debug logs a debug message with optional key-value pairs
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs an info message with optional key-value pairs
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs a warning message with optional key-value pairs
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs an error message with optional key-value pairs
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...any) {
	l.logger.Error(msg, args...)
	os.Exit(1)
}
