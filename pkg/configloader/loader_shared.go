package configloader

import (
	"strings"

	"github.com/hyp3rd/hyperlogger"
)

type rawConfig struct {
	Level           string `mapstructure:"level" yaml:"level"`
	EnableJSON      *bool  `mapstructure:"enable_json" yaml:"enable_json"`
	EnableAsync     *bool  `mapstructure:"enable_async" yaml:"enable_async"`
	AsyncBufferSize *int   `mapstructure:"async_buffer_size" yaml:"async_buffer_size"`
	EncoderName     string `mapstructure:"encoder_name" yaml:"encoder_name"`
	Output          string `mapstructure:"output" yaml:"output"`
	File            struct {
		Path     string `mapstructure:"path" yaml:"path"`
		MaxSize  *int64 `mapstructure:"max_size" yaml:"max_size"`
		Compress *bool  `mapstructure:"compress" yaml:"compress"`
	} `mapstructure:"file" yaml:"file"`
}

func applyRaw(raw rawConfig) (*hyperlogger.Config, error) {
	cfg := hyperlogger.DefaultConfig()

	if raw.Level != "" {
		normalized, err := hyperlogger.ParseLevel(raw.Level)
		if err != nil {
			return nil, err
		}

		cfg.Level = levelFromString(normalized)
	}

	if raw.EnableJSON != nil {
		cfg.EnableJSON = *raw.EnableJSON
	}

	if raw.EnableAsync != nil {
		cfg.EnableAsync = *raw.EnableAsync
	}

	if raw.AsyncBufferSize != nil {
		cfg.AsyncBufferSize = *raw.AsyncBufferSize
	}

	if raw.EncoderName != "" {
		cfg.EncoderName = raw.EncoderName
	}

	if raw.File.Path != "" {
		cfg.File.Path = raw.File.Path
	}

	if raw.File.MaxSize != nil {
		cfg.File.MaxSizeBytes = *raw.File.MaxSize
	}

	if raw.File.Compress != nil {
		cfg.File.Compress = *raw.File.Compress
	}

	if raw.Output != "" {
		writer, err := hyperlogger.SetOutput(raw.Output)
		if err != nil {
			return nil, err
		}

		cfg.Output = writer
	}

	return &cfg, nil
}

func levelFromString(level string) hyperlogger.Level {
	switch strings.ToLower(level) {
	case "trace":
		return hyperlogger.TraceLevel
	case "debug":
		return hyperlogger.DebugLevel
	case "warn":
		return hyperlogger.WarnLevel
	case "error":
		return hyperlogger.ErrorLevel
	case "fatal":
		return hyperlogger.FatalLevel
	default:
		return hyperlogger.InfoLevel
	}
}

func allKeys() []string {
	return []string{
		"level",
		"enable_json",
		"enable_async",
		"async_buffer_size",
		"encoder_name",
		"output",
		"file.path",
		"file.max_size",
		"file.compress",
	}
}
