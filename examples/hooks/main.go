// Package hooks/main demonstrates how to implement and use custom hooks with the logger package.
// It showcases two example hook implementations: a MetricsHook for tracking error/warning
// counts and an AlertHook that simulates sending alerts for critical errors.
//
// This example illustrates:
//   - How to implement the logger.Hook interface
//   - How to create stateful hooks that track metrics
//   - How to register multiple hooks with a logger
//   - How hooks can filter which log levels they respond to

// syncWaitTime is the duration to wait after logging to allow any
// asynchronous logging operations to complete before exiting.

// MetricsHook is a sample hook that tracks error and warning counts.
// It illustrates how to implement a stateful logger hook that can
// maintain metrics about log events.

// OnLog implements logger.Hook interface. It increments internal counters
// when errors or warnings are logged, printing the updated counts to stdout.

// Levels implements logger.Hook interface. It returns the log levels
// this hook should be triggered for (WarnLevel, ErrorLevel, FatalLevel).

// AlertHook demonstrates a hook that could be used to send external alerts
// for critical errors. In a real system, this would integrate with an alerting service.

// OnLog implements logger.Hook interface. It formats and outputs an alert
// message with the log level, message, and any additional fields.

// Levels implements logger.Hook interface. It returns the log levels
// this hook should be triggered for (ErrorLevel, FatalLevel).

// hooks/main demonstrates the creation and use of logger hooks. It:
// 1. Creates instances of MetricsHook and AlertHook
// 2. Configures a logger with these hooks and default settings
// 3. Generates various log messages that trigger the hooks
// 4. Displays the final metrics gathered by the MetricsHook
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	logger "github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/pkg/adapter"
)

const syncWaitTime = 100 * time.Millisecond

// MetricsHook is a sample hook that tracks error counts.
type MetricsHook struct {
	errorCount   int
	warningCount int
}

// OnLog implements logger.Hook.
func (m *MetricsHook) OnLog(entry *logger.Entry) error {
	//nolint:exhaustive
	switch entry.Level {
	case logger.ErrorLevel, logger.FatalLevel:
		m.errorCount++
		fmt.Fprintf(os.Stdout, "Metrics: Error count increased to %d\n", m.errorCount)
	case logger.WarnLevel:
		m.warningCount++
		fmt.Fprintf(os.Stdout, "Metrics: Warning count increased to %d\n", m.warningCount)
	default:
		// Don't handle.
	}

	return nil
}

// Levels implements logger.Hook.
func (*MetricsHook) Levels() []logger.Level {
	return []logger.Level{
		logger.WarnLevel,
		logger.ErrorLevel,
		logger.FatalLevel,
	}
}

// AlertHook sends alerts for critical errors.
type AlertHook struct{}

// OnLog implements logger.Hook.
func (*AlertHook) OnLog(entry *logger.Entry) error {
	// In a real system, this would send to an alerting service
	fmt.Fprintf(os.Stdout, "🚨 ALERT: %s - %s\n", entry.Level, entry.Message)

	// Print any fields
	for _, field := range entry.Fields {
		fmt.Fprintf(os.Stderr, "  %s=%v\n", field.Key, field.Value)
	}

	return nil
}

// Levels implements logger.Hook.
func (*AlertHook) Levels() []logger.Level {
	return []logger.Level{
		logger.ErrorLevel,
		logger.FatalLevel,
	}
}

func main() {
	// Create metrics hook
	metricsHook := &MetricsHook{}

	// Create alert hook
	alertHook := &AlertHook{}

	// Build the configuration
	config := logger.NewConfigBuilder().
		WithConsoleOutput().
		WithDevelopmentDefaults().
		WithField("service", "example-service").
		WithHook("metrics", metricsHook).
		WithHook("alerts", alertHook).
		WithEnableAsync(true).
		Build()

	// Create a logger with the configuration
	log, err := adapter.NewAdapter(context.Background(), *config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)

		return
	}

	defer func() {
		err := log.Sync()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing logger: %v\n", err)
		}
	}()

	// Normal logs - no hooks trigger
	log.Info("Starting service")
	log.Debug("Debug information")

	// These will trigger our hooks
	log.Warn("This is a warning")
	log.Error("An error occurred")

	// Log with context
	log.WithField("user_id", "12345").
		WithField("request_id", "req-abc-123").
		Error("User authentication failed")

	// Wait a moment to allow async operations to complete
	time.Sleep(syncWaitTime)

	// Print final metrics
	fmt.Fprintln(os.Stdout, "\nFinal metrics:")
	fmt.Fprintf(os.Stdout, "  Error count: %d\n", metricsHook.errorCount)
	fmt.Fprintf(os.Stdout, "  Warning count: %d\n", metricsHook.warningCount)
}
