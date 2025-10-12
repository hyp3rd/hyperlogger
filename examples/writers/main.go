package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
	"github.com/hyp3rd/hyperlogger/pkg/adapter"
)

func provideConsoleLogger(ctx context.Context) (hyperlogger.Logger, error) {
	consoleLogger, err := adapter.NewAdapter(ctx, hyperlogger.Config{
		Output: os.Stdout,
		Level:  hyperlogger.InfoLevel,
	})
	if err != nil {
		log.Printf("failed to create console logger: %v\n", err)

		return nil, err
	}

	return consoleLogger, nil
}

func provideFileLogger(ctx context.Context, filePath string) (hyperlogger.Logger, error) {
	fileCfg := hyperlogger.NewConfigBuilder().
		WithFileOutput(filePath).
		WithJSONFormat(true).
		WithEnableAsync(false).
		Build()

	fileLogger, err := adapter.NewAdapter(ctx, *fileCfg)
	if err != nil {
		log.Printf("failed to create file logger: %v\n", err)

		return nil, err
	}

	return fileLogger, nil
}

func provideMultiOutLogger(ctx context.Context) (hyperlogger.Logger, *os.File, error) {
	multiPath := filepath.Join(os.TempDir(), "writers-example-multi.log")

	// Open the file for appending, create it if it doesn't exist
	//nolint:gosec // G304: Potential file inclusion via variable: this is an example, no user input.
	fileHandle, err := os.OpenFile(multiPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("failed to open %s: %v", multiPath, err)

		return nil, nil, ewrap.Wrap(err, "failed to open multi log file")
	}

	multiOutput := io.MultiWriter(os.Stdout, fileHandle)
	multiCfg := hyperlogger.NewConfigBuilder().
		WithOutput(multiOutput).
		WithEnableAsync(false).
		WithJSONFormat(false).
		Build()

	multiLogger, err := adapter.NewAdapter(ctx, *multiCfg)
	if err != nil {
		log.Printf("failed to create multi-writer logger: %v\n", err)

		return nil, nil, err
	}

	return multiLogger, fileHandle, nil
}

func main() {
	ctx := context.Background()

	consoleLogger, err := provideConsoleLogger(ctx)
	if err != nil {
		log.Printf("failed to create console logger: %v\n", err)

		return
	}

	defer func() {
		err := consoleLogger.Sync()
		if err != nil {
			log.Printf("failed to sync console logger: %v", err)
		}
	}()

	consoleLogger.Info("console logger ready")

	filePath := filepath.Join(os.TempDir(), "writers-example.json.log")

	fileLogger, err := provideFileLogger(ctx, filePath)
	if err != nil {
		log.Printf("failed to create file logger: %v\n", err)

		return
	}

	defer func() {
		err := fileLogger.Sync()
		if err != nil {
			log.Printf("failed to sync file logger: %v", err)
		}
	}()

	fileLogger.WithFields(
		hyperlogger.Field{Key: "component", Value: "file-writer"},
		hyperlogger.Field{Key: "path", Value: filePath},
	).Info("this entry is written to a JSON file")

	multiLogger, fileHandle, err := provideMultiOutLogger(ctx)
	if err != nil {
		log.Printf("failed to create multi-writer logger: %v\n", err)

		return
	}

	defer func() {
		err := fileHandle.Close()
		if err != nil {
			log.Printf("failed to close file handle: %v", err)
		}
	}()

	defer func() {
		err := multiLogger.Sync()
		if err != nil {
			log.Printf("failed to sync multi-writer logger: %v", err)
		}
	}()

	multiLogger.Info("this message is mirrored to stdout and a file")
	multiLogger.WithFields(
		hyperlogger.Field{Key: "timestamp", Value: time.Now().Format(time.RFC3339)},
		hyperlogger.Field{Key: "file", Value: fileHandle.Name()},
		hyperlogger.Field{Key: "example", Value: "multi-writer"},
	).Warn("structured output written to multiple destinations")
}
