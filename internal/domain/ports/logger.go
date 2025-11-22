package ports

// Logger defines the interface for structured logging using slog-style key-value pairs
type Logger interface {
	// Debug logs a debug message with optional key-value pairs
	// Example: log.Debug("message", "key1", value1, "key2", value2)
	Debug(msg string, args ...any)

	// Info logs an info message with optional key-value pairs
	// Example: log.Info("message", "key1", value1, "key2", value2)
	Info(msg string, args ...any)

	// Warn logs a warning message with optional key-value pairs
	// Example: log.Warn("message", "key1", value1, "key2", value2)
	Warn(msg string, args ...any)

	// Error logs an error message with optional key-value pairs
	// Example: log.Error("message", "key1", value1, "error", err)
	Error(msg string, args ...any)

	// Fatal logs a fatal message with optional key-value pairs and exits
	// Example: log.Fatal("message", "key1", value1)
	Fatal(msg string, args ...any)
}
