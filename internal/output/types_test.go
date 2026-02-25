package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nopCloser struct {
	*bytes.Buffer
}

func (*nopCloser) Close() error {
	return nil
}

func (*nopCloser) Sync() error {
	return nil
}

func TestWriteResult_Fields(t *testing.T) {
	const expectedBytes = 42

	buf := &bytes.Buffer{}
	closableBuf := &nopCloser{buf}
	wr := WriteResult{
		Writer: closableBuf,
		Name:   "test-writer",
		Bytes:  expectedBytes,
		Err:    nil,
	}

	assert.Equal(t, closableBuf, wr.Writer)
	assert.Equal(t, "test-writer", wr.Name)
	assert.Equal(t, expectedBytes, wr.Bytes)
	assert.NoError(t, wr.Err)
}

func TestColorCode_Values(t *testing.T) {
	const (
		resetCode = 0
		blackCode = 30
		whiteCode = 37
	)

	tests := []struct {
		name     string
		code     ColorCode
		expected int
	}{
		{
			name:     "reset code",
			code:     ColorReset,
			expected: resetCode,
		},
		{
			name:     "black color",
			code:     ColorBlack,
			expected: blackCode,
		},
		{
			name:     "white color",
			code:     ColorWhite,
			expected: whiteCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.code))
		})
	}
}

func TestStyle_Values(t *testing.T) {
	const (
		styleBoldCode      = 1
		styleDimCode       = 2
		styleItalicCode    = 3
		styleUnderlineCode = 4
		styleNormalCode    = 22
	)

	tests := []struct {
		name     string
		style    Style
		expected int
	}{
		{
			name:     "bold style",
			style:    StyleBold,
			expected: styleBoldCode,
		},
		{
			name:     "dim style",
			style:    StyleDim,
			expected: styleDimCode,
		},
		{
			name:     "italic style",
			style:    StyleItalic,
			expected: styleItalicCode,
		},
		{
			name:     "underline style",
			style:    StyleUnderline,
			expected: styleUnderlineCode,
		},
		{
			name:     "normal style",
			style:    StyleNormal,
			expected: styleNormalCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, int(tt.style))
		})
	}
}
