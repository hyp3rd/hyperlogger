package configloader

import (
	"bytes"
	"strings"

	"github.com/hyp3rd/ewrap"
	"github.com/spf13/viper"

	"github.com/hyp3rd/hyperlogger"
)

// FromEnv builds a configuration instance using environment variables with the provided prefix using Viper.
func FromEnv(prefix string) (*hyperlogger.Config, error) {
	viperInstance := viper.New()

	err := configureViperEnv(viperInstance, prefix)
	if err != nil {
		return nil, err
	}

	return fromViper(viperInstance)
}

// FromYAML parses the provided YAML document into a configuration instance using Viper.
func FromYAML(data []byte) (*hyperlogger.Config, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigType("yaml")

	err := viperInstance.ReadConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ewrap.Wrapf(err, "failed to read config from YAML")
	}

	return fromViper(viperInstance)
}

// FromFile loads configuration from the given file path using Viper.
func FromFile(path string) (*hyperlogger.Config, error) {
	viperInstance := viper.New()
	viperInstance.SetConfigFile(path)

	err := viperInstance.ReadInConfig()
	if err != nil {
		return nil, ewrap.Wrapf(err, "failed to read config file %s", path)
	}

	return fromViper(viperInstance)
}

func configureViperEnv(viperInstance *viper.Viper, prefix string) error {
	replacer := strings.NewReplacer(".", "_")
	viperInstance.SetEnvKeyReplacer(replacer)
	viperInstance.AutomaticEnv()

	if prefix != "" {
		viperInstance.SetEnvPrefix(strings.ToLower(strings.TrimSuffix(prefix, "_")))
	}

	errorGroup := ewrap.NewErrorGroup()

	for _, key := range allKeys() {
		err := viperInstance.BindEnv(key)
		if err != nil {
			errorGroup.Add(err)
		}
	}

	if errorGroup.HasErrors() {
		// Log the error but continue; individual keys will be missing if binding fails
		return errorGroup
	}

	return nil
}

func fromViper(viperInstance *viper.Viper) (*hyperlogger.Config, error) {
	var raw rawConfig

	err := viperInstance.Unmarshal(&raw)
	if err != nil {
		return nil, ewrap.Wrapf(err, "failed to unmarshal config")
	}

	return applyRaw(raw)
}
