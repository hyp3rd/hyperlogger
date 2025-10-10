package hyperlogger

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, os.Stdout, config.Output)
	assert.Equal(t, DefaultLevel, config.Level)
	assert.True(t, config.EnableStackTrace)
	assert.True(t, config.EnableCaller)
	assert.Equal(t, DefaultTimeFormat, config.TimeFormat)
	assert.True(t, config.EnableJSON)
	assert.Equal(t, DefaultBufferSize, config.BufferSize)
	assert.Equal(t, DefaultAsyncBufferSize, config.AsyncBufferSize)
	assert.Empty(t, config.AdditionalFields)
}

func TestSetOutput(t *testing.T) {
	// Save current umask and restore it after the test
	// oldUmask := syscall.Umask(0)
	// defer syscall.Umask(oldUmask)
	tests := []struct {
		name        string
		output      string
		wantWriter  io.Writer
		wantErr     bool
		cleanup     func()
		setupFile   bool
		permissions os.FileMode
	}{
		{
			name:       "stdout output",
			output:     "stdout",
			wantWriter: os.Stdout,
			wantErr:    false,
		},
		{
			name:       "stderr output",
			output:     "STDERR",
			wantWriter: os.Stderr,
			wantErr:    false,
		},
		{
			name:        "valid file path",
			output:      filepath.Join(os.TempDir(), "test.log"),
			wantErr:     false,
			setupFile:   true,
			permissions: 0o644, // This is correct - represents real-world permissions
			cleanup: func() {
				os.Remove(filepath.Join(os.TempDir(), "test.log"))
			},
		},
		{
			name:    "invalid file path",
			output:  "/nonexistent/directory/test.log",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			writer, err := SetOutput(tt.output)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, writer)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, writer)

			if tt.wantWriter != nil {
				assert.Equal(t, tt.wantWriter, writer)
			}

			if tt.setupFile {
				file, ok := writer.(*os.File)
				require.True(t, ok)

				info, err := file.Stat()
				require.NoError(t, err)
				assert.Equal(t, tt.permissions, info.Mode().Perm())
				file.Close()
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name        string
		level       string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "debug level",
			level:   "DEBUG",
			want:    "debug",
			wantErr: false,
		},
		{
			name:    "info level",
			level:   "info",
			want:    "info",
			wantErr: false,
		},
		{
			name:    "warn level",
			level:   "Warn",
			want:    "warn",
			wantErr: false,
		},
		{
			name:    "error level",
			level:   "ERROR",
			want:    "error",
			wantErr: false,
		},
		{
			name:    "panic level",
			level:   "panic",
			want:    "panic",
			wantErr: false,
		},
		{
			name:    "fatal level",
			level:   "Fatal",
			want:    "fatal",
			wantErr: false,
		},
		{
			name:        "invalid level",
			level:       "invalid",
			want:        "",
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name:        "empty level",
			level:       "",
			want:        "",
			wantErr:     true,
			errContains: "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLevel(tt.level)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
