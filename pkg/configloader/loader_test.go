package configloader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hyp3rd/hyperlogger"
)

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("APP_LEVEL", "error")
	t.Setenv("APP_ENABLE_JSON", "false")
	t.Setenv("APP_ENABLE_CALLER", "false")
	t.Setenv("APP_ASYNC_BUFFER_SIZE", "2048")
	t.Setenv("APP_COLOR_FORCE_TTY", "true")
	t.Setenv("APP_COLOR_ENABLE", "true")
	t.Setenv("APP_FILE_PATH", "logs/app.log")
	t.Setenv("APP_FILE_MAX_SIZE", "40960")
	t.Setenv("APP_FILE_COMPRESS", "true")
	t.Setenv("APP_SAMPLING_ENABLED", "true")
	t.Setenv("APP_SAMPLING_INITIAL", "10")
	t.Setenv("APP_SAMPLING_THEREAFTER", "5")
	t.Setenv("APP_SAMPLING_RULES_DEBUG_ENABLED", "true")
	t.Setenv("APP_SAMPLING_RULES_DEBUG_INITIAL", "3")
	t.Setenv("APP_SAMPLING_RULES_DEBUG_THEREAFTER", "7")

	cfg, err := FromEnv("app")
	require.NoError(t, err)

	require.Equal(t, hyperlogger.ErrorLevel, cfg.Level)
	require.False(t, cfg.EnableJSON)
	require.False(t, cfg.EnableCaller)
	require.Equal(t, 2048, cfg.AsyncBufferSize)
	require.True(t, cfg.Color.Enable)
	require.True(t, cfg.Color.ForceTTY)
	require.Equal(t, "logs/app.log", cfg.File.Path)
	require.True(t, cfg.File.Compress)
	require.Equal(t, int64(40960), cfg.File.MaxSizeBytes)
	require.True(t, cfg.Sampling.Enabled)
	require.Equal(t, 10, cfg.Sampling.Initial)
	require.Equal(t, 5, cfg.Sampling.Thereafter)
	require.Equal(t, hyperlogger.SamplingRule{Enabled: true, Initial: 3, Thereafter: 7}, cfg.Sampling.Rules[hyperlogger.DebugLevel])
}

func TestFromFileWithEnvOverride(t *testing.T) {
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config.yaml")
	configData := []byte(`
level: debug
enable_json: false
disable_timestamp: true
async:
  buffer_size: 512
  overflow_strategy: block
color:
  enable: false
  level_colors:
    error: "\x1b[31m"
sampling:
  enabled: true
  initial: 7
  thereafter: 3
  rules:
    trace:
      enabled: false
    debug:
      enabled: true
      initial: 4
      thereafter: 2
file:
  path: service.log
  max_size: 1048576
  compress: true
  max_age: 14
  max_backups: 5
`)

	err := os.WriteFile(configPath, configData, 0o600)
	require.NoError(t, err)

	t.Setenv("HYPERLOGGER_LEVEL", "warn")
	t.Setenv("HYPERLOGGER_ENABLE_JSON", "true")

	cfg, err := FromFile(configPath)
	require.NoError(t, err)

	require.Equal(t, hyperlogger.WarnLevel, cfg.Level)
	require.True(t, cfg.EnableJSON)
	require.True(t, cfg.DisableTimestamp)
	require.Equal(t, 512, cfg.AsyncBufferSize)
	require.Equal(t, hyperlogger.AsyncOverflowBlock, cfg.AsyncOverflowStrategy)
	require.False(t, cfg.Color.Enable)
	require.Equal(t, "\x1b[31m", cfg.Color.LevelColors[hyperlogger.ErrorLevel])
	require.True(t, cfg.Sampling.Enabled)
	require.Equal(t, 7, cfg.Sampling.Initial)
	require.Equal(t, 3, cfg.Sampling.Thereafter)
	require.False(t, cfg.Sampling.Rules[hyperlogger.TraceLevel].Enabled)
	require.Equal(t, hyperlogger.SamplingRule{Enabled: true, Initial: 4, Thereafter: 2}, cfg.Sampling.Rules[hyperlogger.DebugLevel])
	require.Equal(t, "service.log", cfg.File.Path)
	require.True(t, cfg.File.Compress)
	require.Equal(t, 14, cfg.File.MaxAge)
	require.Equal(t, 5, cfg.File.MaxBackups)
}

func TestFromYAMLInvalidOverflowStrategy(t *testing.T) {
	data := []byte(`
async:
  overflow_strategy: invalid
`)

	_, err := FromYAML(data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "overflow strategy")
}
