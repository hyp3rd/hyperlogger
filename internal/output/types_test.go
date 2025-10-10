package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nopCloser struct {
	*bytes.Buffer
}

func (nc *nopCloser) Close() error {
	return nil
}

func (nc *nopCloser) Sync() error {
	return nil
}

func TestWriteResult_Fields(t *testing.T) {
	buf := &bytes.Buffer{}
	closableBuf := &nopCloser{buf}
	wr := WriteResult{
		Writer: closableBuf,
		Name:   "test-writer",
		Bytes:  42,
		Err:    nil,
	}

	assert.Equal(t, closableBuf, wr.Writer)
	assert.Equal(t, "test-writer", wr.Name)
	assert.Equal(t, 42, wr.Bytes)
	assert.NoError(t, wr.Err)
}

func TestColorCode_Values(t *testing.T) {
	tests := []struct {
		name     string
		code     ColorCode
		expected int
	}{
		{
			name:     "reset code",
			code:     ColorReset,
			expected: 0,
		},
		{
			name:     "black color",
			code:     ColorBlack,
			expected: 30,
		},
		{
			name:     "white color",
			code:     ColorWhite,
			expected: 37,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.code))
		})
	}
}

func TestStyle_Values(t *testing.T) {
	tests := []struct {
		name     string
		style    Style
		expected int
	}{
		{
			name:     "bold style",
			style:    StyleBold,
			expected: 1,
		},
		{
			name:     "dim style",
			style:    StyleDim,
			expected: 2,
		},
		{
			name:     "italic style",
			style:    StyleItalic,
			expected: 3,
		},
		{
			name:     "underline style",
			style:    StyleUnderline,
			expected: 4,
		},
		{
			name:     "normal style",
			style:    StyleNormal,
			expected: 22,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.style))
		})
	}
}
