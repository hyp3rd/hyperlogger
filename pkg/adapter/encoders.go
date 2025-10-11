package adapter

import (
	"bytes"

	"github.com/hyp3rd/ewrap"

	"github.com/hyp3rd/hyperlogger"
)

const (
	consoleEncoderName = "console"
	jsonEncoderName    = "json"
)

type consoleEncoder struct{}

func (*consoleEncoder) Encode(entry *hyperlogger.Entry, cfg *hyperlogger.Config, buf *bytes.Buffer) ([]byte, error) {
	if entry == nil {
		return nil, ewrap.New("entry cannot be nil")
	}

	if buf == nil {
		buf = bytes.NewBuffer(nil)
	} else {
		buf.Reset()
	}

	formatConsoleOutput(buf, entry, cfg)

	return buf.Bytes(), nil
}

func (*consoleEncoder) EstimateSize(entry *hyperlogger.Entry) int {
	if entry == nil {
		return 0
	}

	return predictBufferSize(false, len(entry.Message), len(entry.Fields))
}

type jsonEncoder struct{}

func (*jsonEncoder) Encode(entry *hyperlogger.Entry, cfg *hyperlogger.Config, buf *bytes.Buffer) ([]byte, error) {
	if entry == nil {
		return nil, ewrap.New("entry cannot be nil")
	}

	if buf == nil {
		buf = bytes.NewBuffer(nil)
	} else {
		buf.Reset()
	}

	formatJSONOutput(buf, entry, cfg)

	return buf.Bytes(), nil
}

func (*jsonEncoder) EstimateSize(entry *hyperlogger.Entry) int {
	if entry == nil {
		return 0
	}

	return predictBufferSize(true, len(entry.Message), len(entry.Fields))
}

func newEncoderRegistryWithDefaults() (*hyperlogger.EncoderRegistry, error) {
	registry := hyperlogger.NewEncoderRegistry()

	err := registerDefaultEncoders(registry)
	if err != nil {
		return nil, err
	}

	return registry, nil
}

func registerDefaultEncoders(registry *hyperlogger.EncoderRegistry) error {
	if registry == nil {
		return ewrap.New("registry cannot be nil")
	}

	if _, exists := registry.Get(consoleEncoderName); !exists {
		err := registry.Register(consoleEncoderName, &consoleEncoder{})
		if err != nil {
			return err
		}
	}

	if _, exists := registry.Get(jsonEncoderName); !exists {
		err := registry.Register(jsonEncoderName, &jsonEncoder{})
		if err != nil {
			return err
		}
	}

	return nil
}
