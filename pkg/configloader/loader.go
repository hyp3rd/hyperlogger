package configloader

import (
	"bytes"
	"strings"

	"github.com/hyp3rd/ewrap"
	"github.com/spf13/viper"

	"github.com/hyp3rd/hyperlogger"
)

const defaultEnvPrefix = "HYPERLOGGER"

// FromEnv loads configuration sourced from environment variables using the provided prefix.
// Environment keys are normalized by uppercasing and replacing dots with underscores.
func FromEnv(prefix string) (*hyperlogger.Config, error) {
	viperInstance := viper.New()

	normalized := normalizePrefix(prefix)

	err := bindEnvironment(viperInstance, normalized)
	if err != nil {
		return nil, err
	}

	raw, err := loadRawFromViper(viperInstance)
	if err != nil {
		return nil, err
	}

	return applyRaw(raw)
}

// FromYAML loads configuration from a YAML document provided as bytes.
func FromYAML(data []byte) (*hyperlogger.Config, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigType("yaml")

	err := viperInstance.ReadConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ewrap.Wrap(err, "failed to read YAML configuration")
	}

	raw, err := loadRawFromViper(viperInstance)
	if err != nil {
		return nil, err
	}

	return applyRaw(raw)
}

// FromFile loads configuration from a YAML file and merges environment overrides using the default prefix.
func FromFile(path string) (*hyperlogger.Config, error) {
	viperInstance := viper.New()

	err := bindEnvironment(viperInstance, defaultEnvPrefix)
	if err != nil {
		return nil, err
	}

	viperInstance.SetConfigFile(path)

	err = viperInstance.ReadInConfig()
	if err != nil {
		return nil, ewrap.Wrap(err, "failed to read configuration file").
			WithMetadata("path", path)
	}

	raw, err := loadRawFromViper(viperInstance)
	if err != nil {
		return nil, err
	}

	return applyRaw(raw)
}

func loadRawFromViper(viperInstance *viper.Viper) (rawConfig, error) {
	var raw rawConfig

	for _, key := range allKeys() {
		if !viperInstance.IsSet(key) {
			continue
		}

		viperInstance.Set(key, viperInstance.Get(key))
	}

	err := viperInstance.Unmarshal(&raw)
	if err != nil {
		return rawConfig{}, ewrap.Wrap(err, "failed to decode configuration")
	}

	return raw, nil
}

func bindEnvironment(viperInstance *viper.Viper, prefix string) error {
	replacer := strings.NewReplacer(".", "_")
	viperInstance.SetEnvKeyReplacer(replacer)

	if prefix != "" {
		viperInstance.SetEnvPrefix(prefix)
	}

	viperInstance.AutomaticEnv()

	for _, key := range allKeys() {
		err := viperInstance.BindEnv(key)
		if err != nil {
			return ewrap.Wrap(err, "failed to bind environment key").
				WithMetadata("key", key).
				WithMetadata("prefix", prefix)
		}
	}

	return nil
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return defaultEnvPrefix
	}

	prefix = strings.TrimSuffix(prefix, "_")
	prefix = strings.ReplaceAll(prefix, "-", "_")

	return strings.ToUpper(prefix)
}
