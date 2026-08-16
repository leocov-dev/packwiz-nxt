package core

import "fmt"

// Logger is an injectable sink for informational/warning messages produced while
// resolving updates, downloading files, or talking to provider APIs. Library
// consumers can supply their own implementation to suppress or redirect this
// output; DefaultRegistry and the default provider clients use PrintLogger,
// which preserves the CLI's historical stdout-printing behavior.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// PrintLogger writes messages to stdout, matching the CLI's historical behavior.
type PrintLogger struct{}

func (PrintLogger) Infof(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (PrintLogger) Warnf(format string, args ...any) {
	fmt.Printf(format, args...)
}

// NoopLogger discards all messages.
type NoopLogger struct{}

func (NoopLogger) Infof(format string, args ...any) {}

func (NoopLogger) Warnf(format string, args ...any) {}
