package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/asmild/bitbucket-pr-reviewer-bot/internal/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Log *logrus.Logger

// Init initializes the logger with configuration
func Init(cfg *config.Config) error {
	Log = logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(cfg.Logging.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	Log.SetLevel(level)

	// Set formatter
	Log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	var writers []io.Writer

	// Console output
	if cfg.Logging.EnableConsole {
		writers = append(writers, os.Stdout)
	}

	// File output with rotation
	if cfg.Logging.EnableFile {
		if err := os.MkdirAll("./logs", 0755); err != nil {
			return fmt.Errorf("failed to create logs directory: %w", err)
		}

		fileWriter := &lumberjack.Logger{
			Filename:   filepath.Join("./logs", fmt.Sprintf("app-%s.log", time.Now().Format("2006-01-02"))),
			MaxSize:    parseMaxFileSize(cfg.Logging.MaxFileSize), // megabytes
			MaxBackups: cfg.Logging.FileRetentionDays,
			MaxAge:     cfg.Logging.FileRetentionDays, // days
			Compress:   true,
		}
		writers = append(writers, fileWriter)
	}

	if len(writers) > 0 {
		Log.SetOutput(io.MultiWriter(writers...))
	}

	return nil
}

// parseMaxFileSize parses size strings like "20m" to megabytes
func parseMaxFileSize(size string) int {
	if len(size) == 0 {
		return 20
	}
	// Simple parser for formats like "20m"
	var mb int
	fmt.Sscanf(size, "%dm", &mb)
	if mb == 0 {
		return 20
	}
	return mb
}

func Info(args ...interface{}) {
	if Log != nil {
		Log.Info(args...)
	}
}

func Infof(format string, args ...interface{}) {
	if Log != nil {
		Log.Infof(format, args...)
	}
}

func Debug(args ...interface{}) {
	if Log != nil {
		Log.Debug(args...)
	}
}

func Debugf(format string, args ...interface{}) {
	if Log != nil {
		Log.Debugf(format, args...)
	}
}

func Warn(args ...interface{}) {
	if Log != nil {
		Log.Warn(args...)
	}
}

func Warnf(format string, args ...interface{}) {
	if Log != nil {
		Log.Warnf(format, args...)
	}
}

func Error(args ...interface{}) {
	if Log != nil {
		Log.Error(args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if Log != nil {
		Log.Errorf(format, args...)
	}
}

func Fatal(args ...interface{}) {
	if Log != nil {
		Log.Fatal(args...)
	}
}

func Fatalf(format string, args ...interface{}) {
	if Log != nil {
		Log.Fatalf(format, args...)
	}
}

func WithFields(fields logrus.Fields) *logrus.Entry {
	if Log != nil {
		return Log.WithFields(fields)
	}
	return nil
}
