// Package log provides application-level logging functionality for services.
//
// This package creates and configures loggers with appropriate settings based on the
// environment (production or non-production) and service name. It offers a simplified
// API for initializing loggers with appropriate defaults:
//
// - In non-production environments: Debug level with readable text output
// - In production environments: Info level with structured JSON output
// - Automatic caller information and stack traces for errors
// - Service name and environment included as additional fields in all log entries
//
// This package is intended to be the primary entry point for applications using the
// logger package, providing a simple way to create well-configured loggers that follow
// best practices for different environments.
//
// Usage:
//
//	log, err := log.New(ctx, "development", "user-service")
//	if err != nil {
//		panic(err)
//	}
//
//	log.Info("Service started successfully")
//	log.WithField("user", userID).Debug("User authenticated")
package log

import (
	"context"
	"os"
	"time"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/internal/constants"
	"github.com/hyp3rd/hyperlogger/internal/output"
	"github.com/hyp3rd/hyperlogger/pkg/adapter"
)

// NewWithDefaults creates a new logger instance with the specified environment and service.
// It configures the logger with a console writer, time format, caller information,
// and additional fields for the service and environment.
// If the environment is non-production, the logger is set to debug level.
// Otherwise, it is set to info level with JSON output.
// The function returns the created logger instance and any error that occurred during initialization.
func NewWithDefaults(ctx context.Context, environment, service string) (hyperlogger.Logger, error) {
	// Create console writer with color support
	consoleWriter := output.NewConsoleWriter(os.Stdout, output.ColorModeAuto)

	// Initialize the logger with proper configuration
	loggerCfg := hyperlogger.DefaultConfig()
	loggerCfg.Output = consoleWriter
	loggerCfg.TimeFormat = time.RFC3339
	loggerCfg.EnableCaller = true
	loggerCfg.EnableStackTrace = true                              // Add this
	loggerCfg.AsyncBufferSize = hyperlogger.DefaultAsyncBufferSize // Explicit buffer size

	if environment == constants.NonProductionEnvironment {
		loggerCfg.Level = hyperlogger.DebugLevel
		loggerCfg.EnableJSON = false
	} else {
		loggerCfg.Level = hyperlogger.InfoLevel
		loggerCfg.EnableJSON = true
	}

	loggerCfg.AdditionalFields = []hyperlogger.Field{
		{Key: "service", Value: service},
		{Key: "environment", Value: environment},
	}

	// Create the logger
	log, err := adapter.NewAdapter(ctx, loggerCfg)
	if err != nil {
		return nil, ewrap.Wrap(err, "failed to create logger")
	}

	return log, nil
}
