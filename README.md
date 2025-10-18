# Hyperlogger

A high-performance, feature-rich logging package for Go applications, focusing on reliability, contextual information, and flexibility.

## Features

- **Multiple Log Levels** - Supports **TRACE**, **DEBUG**, **INFO**, **WARN**, **ERROR**, and **FATAL** log levels
- **Structured Logging** - Add fields and context to your log entries
- **Multiple Output Formats** - Output as text or JSON
- **Color Support** - Terminal-aware color coding based on log levels
- **File Output** - Write logs to files with automatic rotation and compression
- **Performance Optimized**
        - Asynchronous logging with fallback to synchronous for critical logs
        - Optimized buffer pooling with multiple size classes to minimize allocations
        - Caller caching for improved performance
        - Log sampling for high-volume environments
- **Context Integration** - Extract and propagate request context information
- **Comprehensive Error Handling** - Optional stack traces and detailed error reporting
- **Multiple Output Destinations** - Write to console, files, or custom writers
- **Hook System** - Register callbacks for custom processing of log entries
- **Fluent Configuration API** - Simple, chainable configuration with sensible defaults

## Performance

The logger is designed for high performance, with a focus on low latency and minimal memory overhead. It uses a buffered output system to reduce the number of system calls and optimize throughput.

It also implements a sampling mechanism to control log volume in high-throughput scenarios, ensuring that critical logs are captured while reducing the overall load on the system.

### Benchmark Results

```bash
make bench-mem
go test -bench=. -benchtime=3s -benchmem -run=^-memprofile=mem.out ./...
PASS
ok   github.com/hyp3rd/hyperlogger 0.195s
goos: darwin
goarch: arm64
pkg: github.com/hyp3rd/hyperlogger/pkg/adapter
cpu: Apple M2 Pro
BenchmarkAdapterLogging/TextLogging_NoFields-12             4379250         804.9 ns/op         597 B/op            5 allocs/op
BenchmarkAdapterLogging/TextLogging_5Fields-12              4343899         897.8 ns/op         722 B/op            5 allocs/op
BenchmarkAdapterLogging/JSONLogging_NoFields-12             3996409         890.4 ns/op         859 B/op            6 allocs/op
BenchmarkAdapterLogging/JSONLogging_5Fields-12              3378655        1085.0 ns/op        1091 B/op            6 allocs/op
BenchmarkAdapterLogging/TextLogging_MultiWriter-12          4484455         811.8 ns/op         651 B/op            5 allocs/op
PASS

BenchmarkAsyncWriter/drop_newest-12             194167740            18.48 ns/op            24 B/op         1 allocs/op
BenchmarkAsyncWriter/drop_oldest-12              74881224            49.95 ns/op            24 B/op         1 allocs/op
BenchmarkAsyncWriter/block-12                       14628           245278 ns/op           166 B/op         2 allocs/op
BenchmarkAsyncWriter/handoff-12                     29034           124309 ns/op           180 B/op         2 allocs/op
PASS
```

## Installation

```bash
go get -u github.com/hyp3rd/hyperlogger
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"

    logger "github.com/hyp3rd/hyperlogger"
    "github.com/hyp3rd/hyperlogger/adapter"
    "github.com/hyp3rd/hyperlogger/internal/constants"
)

func main() {
    // Create a default logger that writes to stdout
    log, err := adapter.NewAdapter(logger.DefaultConfig())
    if err != nil {
        panic(err)
    }

    // Always call Sync() before application exit to ensure all logs are written
    defer func() {
        err := log.Sync()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
        }
    }()

    // Basic logging
    log.Info("Hello, Logger!")

    // With fields
    log.WithField("user", "john_doe").Info("User logged in")

    // With error
    err = someFunc()
    if err != nil {
        log.WithError(err).Error("Something went wrong")
    }

    logger.RegisterContextExtractor(func(ctx context.Context) []logger.Field {
        if reqID, ok := ctx.Value(constants.RequestKey{}).(string); ok && reqID != "" {
            return []logger.Field{{Key: "request_id", Value: reqID}}
        }

        return nil
    })

    exporter := logger.NewAsyncMetricsExporter()
    logger.RegisterAsyncMetricsHandler(exporter.Observe)
    http.Handle("/metrics", exporter)

    go func() {
        if err := http.ListenAndServe(":9090", nil); err != nil {
            fmt.Fprintf(os.Stderr, "metrics server error: %v\n", err)
        }
    }()
}
```

## Fluent Configuration API

For cleaner, more readable configuration, use the fluent builder API:

```go
// Development logger with console output
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithConsoleOutput().
    WithDevelopmentDefaults().
    WithField("service", "auth-service").
    Build())

// Production logger with file output and rotation
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithFileOutput("/var/log/app.log").
    WithFileRotation(100*1024*1024, true). // 100MB with compression
    WithFileRetention(7, 10). // 7 days, max 10 backup files
    WithProductionDefaults().
    WithAsyncLogging(true, 1024). // Enable async logging with buffer size
    Build())
```

## Application Logger

For applications, you can use the app package which provides environment-aware logger initialization:

```go
package main

import (
    "context"
    "fmt"
    "os"
    logger "github.com/hyp3rd/hyperlogger/log"
)

func main() {
    // Create cancellable context
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Create an application logger with environment and service information
    log, err := logger.New(ctx, "development", "my-service")
    if err != nil {
        panic(err)
    }

    defer func() {
        err := log.Sync()
        if err != nil {
            fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
        }
    }()

    // In development: Text output with debug level
    // In production: JSON output with info level

    log.Info("Application started")

    log.WithError(fmt.Errorf("failed to create client")).Fatal("Failed to create client")

    log.WithFields(logger.Field{
        Key:   "metadata",
        Value: "example",
    }).Info("Processing request")
}
```

## Configuration

### Logging Levels

The logger supports multiple log levels, allowing you to control the verbosity of your logs. The available log levels are:

```go
logger.TraceLevel // Most verbose
logger.DebugLevel // Debug information
logger.InfoLevel  // General operational information
logger.WarnLevel  // Warnings
logger.ErrorLevel // Errors
logger.FatalLevel // Fatal errors
```

### Output Configuration

```go
// Using traditional configuration
config := logger.DefaultConfig()

// Configure output destination
config.Output = os.Stdout  // Standard output
// Or use file output with rotation and compression
fileWriter, err := output.NewFileWriter(output.FileConfig{
    Path:     "/var/log/my_app.log",
    MaxSize:  100 * 1024 * 1024, // 100MB
    Compress: true,
    CompressionConfig: &output.CompressionConfig{
        Level: output.CompressionSpeed, // Optimize for speed
    },
})
if err != nil {
    panic(err)
}
config.Output = fileWriter

// Or use multiple outputs
multiWriter, err := output.NewMultiWriter(
    output.NewConsoleWriter(os.Stdout, output.ColorModeAuto),
    fileWriter,
)
if err != nil {
    panic(err)
}
config.Output = multiWriter

// Set log level
config.Level = logger.InfoLevel

// Configure colors
config.Color.Enable = true
config.Color.ForceTTY = false // Only use colors when output is a TTY

// Format settings
config.EnableJSON = false // Use text format (default is JSON)
config.EnableCaller = true // Include caller information
config.EnableStackTrace = true // Include stack traces for errors
config.TimeFormat = time.RFC3339 // Set timestamp format

// Configure asynchronous logging
config.EnableAsync = true // Use non-blocking asynchronous logging
config.AsyncBufferSize = 1024 // Buffer size for async logging

// Configure log sampling for high-volume scenarios
config.Sampling = logger.SamplingConfig{
    Enabled:          true,
    Initial:          1000,   // Log first 1000 entries normally
    Thereafter:       100,    // Then log only 1/100 entries
    PerLevelThreshold: true,  // Apply thresholds per level
}
```

### Create a Logger

```go
// With the adapter directly
log, err := adapter.NewAdapter(config)
if err != nil {
    panic(err)
}

// Or use a no-op logger for testing
noopLog := logger.NewNoop()
```

## API Reference

### Basic Logging Methods

```go
log.Trace("Detailed debugging information")
log.Debug("Debugging information")
log.Info("General information")
log.Warn("Warning message")
log.Error("Error message")
log.Fatal("Fatal error message")
```

### Formatted Logging

```go
log.Tracef("User %s made %d requests", username, count)
log.Debugf("Processing took %v", duration)
log.Infof("Server started on port %d", port)
log.Warnf("High memory usage: %.2f%%", memoryPercent)
log.Errorf("Failed to connect to %s: %v", host, err)
log.Fatalf("Critical system failure: %v", err)
```

### Contextual Logging

```go
// With request context
log = log.WithContext(ctx)

// With fields
log = log.WithField("user_id", userID)
log = log.WithFields(
    logger.Field{Key: "request_id", Value: reqID},
    logger.Field{Key: "client_ip", Value: clientIP},
)

// With error information
log = log.WithError(err)
```

```go
// Register a global extractor that enriches every logger with trace metadata
logger.RegisterContextExtractor(func(ctx context.Context) []logger.Field {
    if traceID, ok := ctx.Value(constants.TraceKey{}).(string); ok && traceID != "" {
        return []logger.Field{{Key: "trace_id", Value: traceID}}
    }
    return nil
})
```

Global extractors run in addition to per-logger extractors configured through the builder, making it easy to centralize cross-cutting enrichment logic.

### Sync - Ensuring Logs are Written

When using asynchronous logging, it's crucial to call the `Sync()` method before your application exits to ensure all buffered logs are written:

```go
// Make sure all logs are written before exit
err := log.Sync()
if err != nil {
    fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
}
```

For convenience, you can use a defer statement at the start of your `main()` function:

```go
defer func() {
    err := log.Sync()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
    }
}()
```

## Hook System

The logger supports hooks for custom processing of log entries:

```go
// Create a metrics hook
metricsHook := &MetricsHook{}

// Create an alert hook
alertHook := &AlertHook{}

// Build a logger with hooks
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithConsoleOutput().
    WithField("service", "auth-service").
    WithHook("metrics", metricsHook).
    WithHook("alerts", alertHook).
    Build())
```

Example hook implementation:

```go
// MetricsHook is a sample hook that tracks error counts.
type MetricsHook struct {
    errorCount int
}

// OnLog implements logger.Hook interface
func (m *MetricsHook) OnLog(entry *logger.Entry) error {
    if entry.Level == logger.ErrorLevel {
        m.errorCount++
        // Send to metrics system...
    }
    return nil
}

// Levels implements logger.Hook interface
func (m *MetricsHook) Levels() []logger.Level {
    return []logger.Level{logger.ErrorLevel, logger.FatalLevel}
}

// Register a global error hook
logger.RegisterHook(logger.ErrorLevel, func(ctx context.Context, entry *logger.Entry) error {
    errorCounter.Inc()
    return nil
})
```

## Advanced Features

### Asynchronous Logging

Asynchronous logging decouples logging operations from your application's main execution path, significantly improving performance:

```go
// Enable asynchronous logging with a buffer size of 1024
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithConsoleOutput().
    WithAsyncLogging(true, 1024).
    Build())
```

The AsyncWriter implementation provides:

- Non-blocking write operations (falls back to error when buffer is full)
- Buffered channel for high throughput
- Proper cleanup with the `Sync()` method
- Configurable buffer size and timeout
- Error handling for failed write operations
- Multiple overflow strategies (`drop_newest`, `drop_oldest`, `block`, `handoff`)
- Metrics callbacks that surface queue depth, drop counts, synchronous bypasses, and write failures

```go
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithConsoleOutput().
    WithAsyncLogging(true, 1024).
    WithAsyncOverflowStrategy(logger.AsyncOverflowHandoff).
    WithAsyncMetricsHandler(exporter.Observe).
    Build())
```

#### Async Metrics Exporter

Expose async writer health metrics via a Prometheus-style endpoint:

```go
exporter := logger.NewAsyncMetricsExporter()
logger.RegisterAsyncMetricsHandler(exporter.Observe)
http.Handle("/metrics", exporter)
```

This exporter tracks enqueued, processed, dropped, retried, and bypassed logs as well as the current queue depth.

### Log Sampling

When dealing with high-volume logging, you can enable sampling to prevent overwhelming your system:

```go
// Allow first 1000 logs at each level, then only log 1/100
b := logger.NewConfigBuilder().
    WithConsoleOutput().
    WithSampling(true, 1000, 100, true).
    WithSamplingRule(logger.DebugLevel, logger.SamplingRule{Enabled: true, Initial: 200, Thereafter: 10}).
    WithSamplingRule(logger.TraceLevel, logger.SamplingRule{Enabled: false})

log, err := adapter.NewAdapter(*b.Build())
```

Per-level sampling rules keep critical log levels verbosely while aggressively sampling noisy ones.

### Middleware Helpers

Seed context values automatically using the provided HTTP and gRPC middleware:

```go
// HTTP middleware enriches context with request identifiers.
handler := httpmw.ContextMiddleware()(nextHandler)

// gRPC interceptor (build with -tags grpc) maps incoming metadata into context.
interceptor := grpcmw.UnaryServerInterceptor(
    grpcmw.WithTraceKey("x-trace"),
    grpcmw.WithRequestKey("x-request"),
)
grpc.NewServer(grpc.UnaryInterceptor(interceptor))
```

Downstream logging code can then access these values via context extractors or the built-in key mappings.

### Async Benchmarks

Evaluate how overflow strategies behave under sustained load:

```bash
go test -bench BenchmarkAsyncWriter -benchmem ./internal/output
```

This benchmark introduces artificial back-pressure to surface throughput differences between the drop, block, and handoff strategies.

### Enhanced File Management

Control compression and retention policies for log files:

```go
log, err := adapter.NewAdapter(*logger.NewConfigBuilder().
    WithFileOutput("/var/log/app.log").
    WithFileRotation(100*1024*1024, true).
    WithFileCompression(1). // Fast compression (1=speed, 9=best compression)
    WithFileRetention(7, 10). // Keep logs for 7 days, max 10 files
    Build())
```

## Performance Considerations

- **Buffer Optimization**: The logger uses multiple buffer size classes (512B to 64KB) for efficient memory use
- **Sampling Control**: Reduce log volume in high-throughput environments while ensuring critical logs are captured
- **Async Processing**: Logs are processed asynchronously by default, with fallback to synchronous for critical logs
- **Memory Management**: Proper pooling of buffers and field slices to reduce garbage collection pressure
- **Caller Caching**: File and line information is cached to reduce runtime.Caller overhead
- **Efficient Compression**: Configurable compression with buffer pooling for rotated log files

When using the logger in performance-critical applications:

- Use appropriate log levels to avoid unnecessary processing
- Enable sampling for high-volume logs that aren't critical
- Always call the `Sync()` method before application exit to ensure all buffered logs are written
- Consider using the JSON format in production for more efficient parsing
- Use async logging for improved performance, but remember to call `Sync()` before exit

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.
