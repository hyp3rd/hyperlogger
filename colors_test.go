package hyperlogger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultLevelColors(t *testing.T) {
	colors := DefaultLevelColors()

	assert.Equal(t, Magenta, colors[TraceLevel])
	assert.Equal(t, Blue, colors[DebugLevel])
	assert.Equal(t, Green, colors[InfoLevel])
	assert.Equal(t, Yellow, colors[WarnLevel])
	assert.Equal(t, Red, colors[ErrorLevel])
	assert.Equal(t, BoldRed, colors[FatalLevel])
	assert.Len(t, colors, 6)
}

func TestDefaultColorConfig(t *testing.T) {
	config := DefaultColorConfig()

	assert.True(t, config.Enable)
	assert.True(t, config.ForceTTY)
	assert.Equal(t, DefaultLevelColors(), config.LevelColors)
}

func TestColorConstants(t *testing.T) {
	tests := []struct {
		name     string
		color    string
		expected string
	}{
		{"Black", Black, "\x1b[30m"},
		{"Red", Red, "\x1b[31m"},
		{"Green", Green, "\x1b[32m"},
		{"Yellow", Yellow, "\x1b[33m"},
		{"Blue", Blue, "\x1b[34m"},
		{"Magenta", Magenta, "\x1b[35m"},
		{"Cyan", Cyan, "\x1b[36m"},
		{"White", White, "\x1b[37m"},
		{"BoldBlack", BoldBlack, "\x1b[30;1m"},
		{"BoldRed", BoldRed, "\x1b[31;1m"},
		{"BoldGreen", BoldGreen, "\x1b[32;1m"},
		{"BoldYellow", BoldYellow, "\x1b[33;1m"},
		{"BoldBlue", BoldBlue, "\x1b[34;1m"},
		{"BoldMagenta", BoldMagenta, "\x1b[35;1m"},
		{"BoldCyan", BoldCyan, "\x1b[36;1m"},
		{"BoldWhite", BoldWhite, "\x1b[37;1m"},
		{"Reset", Reset, "\x1b[0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.color)
		})
	}
}
