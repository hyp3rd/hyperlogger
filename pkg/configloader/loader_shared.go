package configloader

import (
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
)

const fileModeBitSize = 32

type rawConfig struct {
	Level            string `mapstructure:"level"`
	TimeFormat       string `mapstructure:"time_format"`
	EnableJSON       *bool  `mapstructure:"enable_json"`
	DisableTimestamp *bool  `mapstructure:"disable_timestamp"`
	EnableCaller     *bool  `mapstructure:"enable_caller"`
	EnableStackTrace *bool  `mapstructure:"enable_stack_trace"`
	EnableAsync      *bool  `mapstructure:"enable_async"`
	Async            struct {
		BufferSize       *int   `mapstructure:"buffer_size"`
		OverflowStrategy string `mapstructure:"overflow_strategy"`
	} `mapstructure:"async"`
	BufferSize       *int                  `mapstructure:"buffer_size"`
	Encoder          struct{ Name string } `mapstructure:"encoder"`
	EncoderName      string                `mapstructure:"encoder_name"`
	Output           string                `mapstructure:"output"`
	AdditionalFields map[string]any        `mapstructure:"additional_fields"`
	Color            rawColorConfig        `mapstructure:"color"`
	Sampling         rawSamplingConfig     `mapstructure:"sampling"`
	File             rawFileConfig         `mapstructure:"file"`
}

func applyRaw(raw rawConfig) (*hyperlogger.Config, error) {
	cfg := hyperlogger.DefaultConfig()

	err := applyCoreSettings(&cfg, raw)
	if err != nil {
		return nil, err
	}

	err = applyAsyncSettings(&cfg, raw)
	if err != nil {
		return nil, err
	}

	applyEncoderSettings(&cfg, raw)

	err = applyFileSettings(&cfg, raw.File)
	if err != nil {
		return nil, err
	}

	err = applyOutputDestination(&cfg, raw.Output)
	if err != nil {
		return nil, err
	}

	if len(raw.AdditionalFields) > 0 {
		cfg.AdditionalFields = convertAdditionalFields(raw.AdditionalFields)
	}

	err = applyColorConfig(&cfg.Color, raw.Color)
	if err != nil {
		return nil, err
	}

	err = applySampling(&cfg.Sampling, raw.Sampling)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyCoreSettings(cfg *hyperlogger.Config, raw rawConfig) error {
	if raw.Level != "" {
		level, err := parseLevel(raw.Level)
		if err != nil {
			return err
		}

		cfg.Level = level
	}

	if raw.TimeFormat != "" {
		cfg.TimeFormat = raw.TimeFormat
	}

	setBool(&cfg.EnableJSON, raw.EnableJSON)
	setBool(&cfg.DisableTimestamp, raw.DisableTimestamp)
	setBool(&cfg.EnableCaller, raw.EnableCaller)
	setBool(&cfg.EnableStackTrace, raw.EnableStackTrace)

	if raw.BufferSize != nil {
		cfg.BufferSize = *raw.BufferSize
	}

	return nil
}

func applyAsyncSettings(cfg *hyperlogger.Config, raw rawConfig) error {
	setBool(&cfg.EnableAsync, raw.EnableAsync)

	if raw.Async.BufferSize != nil {
		cfg.AsyncBufferSize = *raw.Async.BufferSize
	}

	if raw.Async.OverflowStrategy == "" {
		return nil
	}

	strategy, err := parseOverflowStrategy(raw.Async.OverflowStrategy)
	if err != nil {
		return err
	}

	cfg.AsyncOverflowStrategy = strategy

	return nil
}

func applyEncoderSettings(cfg *hyperlogger.Config, raw rawConfig) {
	if raw.EncoderName != "" {
		cfg.EncoderName = raw.EncoderName
	}

	if raw.Encoder.Name != "" {
		cfg.EncoderName = raw.Encoder.Name
	}
}

func applyFileSettings(cfg *hyperlogger.Config, file rawFileConfig) error {
	if file.Path != "" {
		cfg.File.Path = file.Path
		cfg.FilePath = file.Path
	}

	if file.MaxSize != nil {
		cfg.File.MaxSizeBytes = *file.MaxSize
		cfg.FileMaxSize = *file.MaxSize
	}

	if file.Compress != nil {
		cfg.File.Compress = *file.Compress
		cfg.FileCompress = *file.Compress
	}

	if file.MaxAge != nil {
		cfg.File.MaxAge = *file.MaxAge
	}

	if file.MaxBackups != nil {
		cfg.File.MaxBackups = *file.MaxBackups
	}

	setBool(&cfg.File.LocalTime, file.LocalTime)

	if file.FileMode != "" {
		mode, err := parseFileMode(file.FileMode)
		if err != nil {
			return err
		}

		cfg.File.FileMode = mode
	}

	if file.CompressionLevel != nil {
		cfg.File.CompressionLevel = *file.CompressionLevel
	}

	return nil
}

func applyOutputDestination(cfg *hyperlogger.Config, output string) error {
	if output == "" {
		return nil
	}

	writer, err := hyperlogger.SetOutput(output)
	if err != nil {
		return err
	}

	cfg.Output = writer

	return nil
}

func setBool(target *bool, value *bool) {
	if value == nil {
		return
	}

	*target = *value
}

type rawColorConfig struct {
	Enable      *bool             `mapstructure:"enable"`
	ForceTTY    *bool             `mapstructure:"force_tty"`
	LevelColors map[string]string `mapstructure:"level_colors"`
}

type rawSamplingConfig struct {
	Enabled           *bool                      `mapstructure:"enabled"`
	Initial           *int                       `mapstructure:"initial"`
	Thereafter        *int                       `mapstructure:"thereafter"`
	PerLevelThreshold *bool                      `mapstructure:"per_level_threshold"`
	Rules             map[string]rawSamplingRule `mapstructure:"rules"`
}

type rawSamplingRule struct {
	Enabled    *bool `mapstructure:"enabled"`
	Initial    *int  `mapstructure:"initial"`
	Thereafter *int  `mapstructure:"thereafter"`
}

type rawFileConfig struct {
	Path             string `mapstructure:"path"`
	MaxSize          *int64 `mapstructure:"max_size"`
	Compress         *bool  `mapstructure:"compress"`
	MaxAge           *int   `mapstructure:"max_age"`
	MaxBackups       *int   `mapstructure:"max_backups"`
	LocalTime        *bool  `mapstructure:"local_time"`
	FileMode         string `mapstructure:"file_mode"`
	CompressionLevel *int   `mapstructure:"compression_level"`
}

func parseLevel(level string) (hyperlogger.Level, error) {
	normalized, err := hyperlogger.ParseLevel(level)
	if err != nil {
		return hyperlogger.InfoLevel, err
	}

	return levelFromString(normalized)
}

func levelFromString(level string) (hyperlogger.Level, error) {
	switch strings.ToLower(level) {
	case "trace":
		return hyperlogger.TraceLevel, nil
	case "debug":
		return hyperlogger.DebugLevel, nil
	case "info":
		return hyperlogger.InfoLevel, nil
	case "warn":
		return hyperlogger.WarnLevel, nil
	case "error":
		return hyperlogger.ErrorLevel, nil
	case "fatal":
		return hyperlogger.FatalLevel, nil
	default:
		return hyperlogger.InfoLevel, ewrap.New("unsupported log level").
			WithMetadata("level", level)
	}
}

func parseOverflowStrategy(value string) (hyperlogger.AsyncOverflowStrategy, error) {
	switch strings.ToLower(value) {
	case "drop_newest", "drop-newest", "":
		return hyperlogger.AsyncOverflowDropNewest, nil
	case "block":
		return hyperlogger.AsyncOverflowBlock, nil
	case "drop_oldest", "drop-oldest":
		return hyperlogger.AsyncOverflowDropOldest, nil
	case "handoff":
		return hyperlogger.AsyncOverflowHandoff, nil
	default:
		return hyperlogger.AsyncOverflowDropNewest, ewrap.New("unsupported async overflow strategy").
			WithMetadata("strategy", value)
	}
}

func convertAdditionalFields(entries map[string]any) []hyperlogger.Field {
	if len(entries) == 0 {
		return nil
	}

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	fields := make([]hyperlogger.Field, 0, len(entries))
	for _, key := range keys {
		fields = append(fields, hyperlogger.Field{Key: key, Value: entries[key]})
	}

	return fields
}

func applyColorConfig(cfg *hyperlogger.ColorConfig, raw rawColorConfig) error {
	if cfg == nil {
		return ewrap.New("color config cannot be nil")
	}

	if raw.Enable != nil {
		cfg.Enable = *raw.Enable
	}

	if raw.ForceTTY != nil {
		cfg.ForceTTY = *raw.ForceTTY
	}

	if len(raw.LevelColors) == 0 {
		return nil
	}

	for key, value := range raw.LevelColors {
		level, err := levelFromString(key)
		if err != nil {
			return err
		}

		cfg.LevelColors[level] = value
	}

	return nil
}

func applySampling(cfg *hyperlogger.SamplingConfig, raw rawSamplingConfig) error {
	if cfg == nil {
		return ewrap.New("sampling config cannot be nil")
	}

	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	}

	if raw.Initial != nil {
		cfg.Initial = *raw.Initial
	}

	if raw.Thereafter != nil {
		cfg.Thereafter = *raw.Thereafter
	}

	if raw.PerLevelThreshold != nil {
		cfg.PerLevelThreshold = *raw.PerLevelThreshold
	}

	return applySamplingRules(cfg, raw.Rules)
}

func applySamplingRules(cfg *hyperlogger.SamplingConfig, rawRules map[string]rawSamplingRule) error {
	if len(rawRules) == 0 {
		return nil
	}

	if cfg.Rules == nil {
		cfg.Rules = make(map[hyperlogger.Level]hyperlogger.SamplingRule)
	}

	for key, rule := range rawRules {
		level, err := levelFromString(key)
		if err != nil {
			return err
		}

		enabled := true
		if rule.Enabled != nil {
			enabled = *rule.Enabled
		}

		initial := cfg.Initial
		if rule.Initial != nil {
			initial = *rule.Initial
		}

		thereafter := cfg.Thereafter
		if rule.Thereafter != nil {
			thereafter = *rule.Thereafter
		}

		cfg.Rules[level] = hyperlogger.SamplingRule{
			Enabled:    enabled,
			Initial:    initial,
			Thereafter: thereafter,
		}
	}

	return nil
}

func parseFileMode(value string) (os.FileMode, error) {
	if value == "" {
		return 0, ewrap.New("file mode cannot be empty")
	}

	parsed, err := strconv.ParseUint(value, 0, fileModeBitSize)
	if err != nil {
		return 0, ewrap.Wrap(err, "invalid file mode").WithMetadata("value", value)
	}

	return os.FileMode(parsed), nil
}

func allKeys() []string {
	return []string{
		"level",
		"time_format",
		"enable_json",
		"disable_timestamp",
		"enable_caller",
		"enable_stack_trace",
		"buffer_size",
		"enable_async",
		"async.buffer_size",
		"async.overflow_strategy",
		"encoder_name",
		"encoder.name",
		"output",
		"color.enable",
		"color.force_tty",
		"sampling.enabled",
		"sampling.initial",
		"sampling.thereafter",
		"sampling.per_level_threshold",
		"sampling.rules.trace.enabled",
		"sampling.rules.trace.initial",
		"sampling.rules.trace.thereafter",
		"sampling.rules.debug.enabled",
		"sampling.rules.debug.initial",
		"sampling.rules.debug.thereafter",
		"sampling.rules.info.enabled",
		"sampling.rules.info.initial",
		"sampling.rules.info.thereafter",
		"sampling.rules.warn.enabled",
		"sampling.rules.warn.initial",
		"sampling.rules.warn.thereafter",
		"sampling.rules.error.enabled",
		"sampling.rules.error.initial",
		"sampling.rules.error.thereafter",
		"sampling.rules.fatal.enabled",
		"sampling.rules.fatal.initial",
		"sampling.rules.fatal.thereafter",
		"file.path",
		"file.max_size",
		"file.compress",
		"file.max_age",
		"file.max_backups",
		"file.local_time",
		"file.file_mode",
		"file.compression_level",
	}
}
