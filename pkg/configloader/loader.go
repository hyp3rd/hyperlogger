package configloader

import (
	"bytes"
	"os"
	"strconv"
	"strings"

	"github.com/hyp3rd/ewrap"
	"gopkg.in/yaml.v3"

	"github.com/hyp3rd/hyperlogger"
)

const bitSize = 64

// FromEnv loads configuration from environment variables with the given prefix.
// If prefix is empty, "HYPERLOGGER_" is used.
// Environment variable names are constructed by converting configuration keys to uppercase
// and replacing dots with underscores. For example, the key "file.max_size" becomes "HYPERLOGGER_FILE_MAX_SIZE".
func FromEnv(prefix string) (*hyperlogger.Config, error) {
	raw := rawConfig{}
	prefix = normalizePrefix(prefix)

	for _, key := range allKeys() {
		if value, ok := lookupEnv(prefix, key); ok {
			err := bindEnvValue(&raw, key, value)
			if err != nil {
				return nil, err
			}
		}
	}

	return applyRaw(raw)
}

// FromYAML loads configuration from a YAML byte slice.
func FromYAML(data []byte) (*hyperlogger.Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))

	var raw rawConfig

	err := dec.Decode(&raw)
	if err != nil {
		return nil, ewrap.Wrapf(err, "failed to decode YAML configuration")
	}

	return applyRaw(raw)
}

// FromFile loads configuration from a YAML file at the specified path.
func FromFile(path string) (*hyperlogger.Config, error) {
	//nolint:gosec // the config file path is not provided by untrusted input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ewrap.Wrap(err, "failed to read configuration file").
			WithMetadata("file_path", path)
	}

	return FromYAML(data)
}

func normalizePrefix(prefix string) string {
	if prefix == "" {
		return "HYPERLOGGER_"
	}

	if prefix[len(prefix)-1] != '_' {
		prefix += "_"
	}

	return prefix
}

func lookupEnv(prefix, key string) (string, bool) {
	envKey := prefix + toEnvKey(key)

	return os.LookupEnv(envKey)
}

func toEnvKey(key string) string {
	return strings.ToUpper(strings.ReplaceAll(key, ".", "_"))
}

//nolint:cyclop,revive // it's okay for this function to be a bit complex
func bindEnvValue(raw *rawConfig, key, value string) error {
	switch key {
	case "level":
		raw.Level = value
	case "enable_json":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return ewrap.Wrap(err, "failed to parse boolean").
				WithMetadata("key", key)
		}

		raw.EnableJSON = &b
	case "enable_async":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return ewrap.Wrap(err, "failed to parse boolean").
				WithMetadata("key", key)
		}

		raw.EnableAsync = &b
	case "async_buffer_size":
		i, err := strconv.Atoi(value)
		if err != nil {
			return ewrap.Wrap(err, "failed to parse integer").
				WithMetadata("key", key)
		}

		raw.AsyncBufferSize = &i
	case "encoder_name":
		raw.EncoderName = value
	case "output":
		raw.Output = value
	case "file.path":
		raw.File.Path = value
	case "file.max_size":
		v, err := strconv.ParseInt(value, 10, bitSize)
		if err != nil {
			return ewrap.Wrap(err, "failed to parse integer").
				WithMetadata("key", key)
		}

		raw.File.MaxSize = &v
	case "file.compress":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return ewrap.Wrap(err, "failed to parse boolean").
				WithMetadata("key", key)
		}

		raw.File.Compress = &b

	default:
		return ewrap.Newf("unknown configuration key").
			WithMetadata("key", key)
	}

	return nil
}
