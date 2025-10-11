package output

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logger "github.com/hyp3rd/hyperlogger"
)

type closableBuffer struct {
	*bytes.Buffer
	closed bool
}

func (c *closableBuffer) Close() error {
	c.closed = true

	return nil
}

func TestNewWriterAdapter(t *testing.T) {
	cb := &closableBuffer{Buffer: bytes.NewBuffer(nil)}
	adapter := NewWriterAdapter(cb)

	_, err := adapter.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, "hello", cb.String())

	assert.NoError(t, adapter.Sync(), "sync should succeed even if unsupported")
	assert.NoError(t, adapter.Close())
	assert.True(t, cb.closed, "close should be forwarded to underlying writer")
}

func TestNewFileWriter(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name        string
		config      FileConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: FileConfig{
				Path:     logPath,
				MaxSize:  1024,
				Compress: true,
				FileMode: 0o644,
			},
			expectError: false,
		},
		{
			name: "empty path",
			config: FileConfig{
				Path: "",
			},
			expectError: true,
		},
		{
			name: "default values",
			config: FileConfig{
				Path: logPath,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := NewFileWriter(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, writer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, writer)
				writer.Close()
			}
		})
	}
}

func TestFileWriter_Write(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	writer, err := NewFileWriter(FileConfig{
		Path:     logPath,
		MaxSize:  100,
		Compress: false,
	})
	require.NoError(t, err)

	defer writer.Close()

	testData := []byte("test log entry\n")
	n, err := writer.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	content, err := os.ReadFile(logPath)
	assert.NoError(t, err)
	assert.Equal(t, testData, content)
}

func TestConsoleWriter_Write(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewConsoleWriter(buf, ColorModeAlways)

	tests := []struct {
		name    string
		input   []byte
		level   logger.Level
		wantErr bool
	}{
		{
			name:    "info level",
			input:   []byte("INFO test message"),
			level:   logger.InfoLevel,
			wantErr: false,
		},
		{
			name:    "error level",
			input:   []byte("ERROR test message"),
			level:   logger.ErrorLevel,
			wantErr: false,
		},
		{
			name:    "debug level",
			input:   []byte("DEBUG test message"),
			level:   logger.DebugLevel,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()

			n, err := writer.Write(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.input), n)
				assert.NotEmpty(t, buf.String())
			}
		})
	}
}

func TestMultiWriter_WriteToWriters(t *testing.T) {
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	writer1 := NewConsoleWriter(buf1, ColorModeNever)
	writer2 := NewConsoleWriter(buf2, ColorModeNever)

	multi, err := NewMultiWriter(writer1, writer2)
	require.NoError(t, err)

	testData := []byte("test message\n")
	n, err := multi.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	assert.Equal(t, testData, buf1.Bytes())
	assert.Equal(t, testData, buf2.Bytes())
}

func TestMultiWriter_AddRemoveWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewConsoleWriter(buf, ColorModeNever)

	multi, err := NewMultiWriter(writer)
	require.NoError(t, err)

	newBuf := &bytes.Buffer{}
	newWriter := NewConsoleWriter(newBuf, ColorModeNever)

	err = multi.AddWriter(newWriter)
	assert.NoError(t, err)
	assert.Len(t, multi.Writers, 2)

	multi.RemoveWriter(writer)
	assert.Len(t, multi.Writers, 1)

	testData := []byte("test message\n")
	_, err = multi.Write(testData)
	assert.NoError(t, err)
	assert.Equal(t, testData, newBuf.Bytes())
}

func TestFileWriter_Rotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "rotation.log")

	writer, err := NewFileWriter(FileConfig{
		Path:     logPath,
		MaxSize:  10,
		Compress: false,
	})
	require.NoError(t, err)

	defer writer.Close()

	// Write enough data to trigger rotation
	data := []byte("test data that will cause rotation\n")
	_, err = writer.Write(data)
	assert.NoError(t, err)

	// Give rotation a moment to complete
	time.Sleep(100 * time.Millisecond)

	// Check that rotation created a backup file
	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Len(t, files, 2)
}

// func TestMultiWriter_ConcurrentAccess(t *testing.T) {
// 	buf1 := &bytes.Buffer{}
// 	buf2 := &bytes.Buffer{}
// 	writer1 := NewConsoleWriter(buf1, ColorModeNever)
// 	writer2 := NewConsoleWriter(buf2, ColorModeNever)

// 	multi, err := NewMultiWriter(writer1, writer2)
// 	require.NoError(t, err)

// 	const numGoroutines = 10
// 	const numWrites = 100

// 	// Channel to collect all written messages
// 	:= make(chan string, numGoroutines*numWrites)

// 	var wg sync.WaitGroup
// 	wg.Add(numGoroutines)

// 	// Write messages
// 	for i := 0; i < numGoroutines; i++ {
// 		go func(id int) {
// 			defer wg.Done()
// 			for j := 0; j < numWrites; j++ {
// 				msg := fmt.Sprintf("test message %d-%d\n", id, j)
// 				messages <- msg
// 				_, err := multi.Write([]byte(msg))
// 				assert.NoError(t, err)
// 			}
// 		}(i)
// 	}

// 	// Close channel after all writes complete
// 	go func() {
// 		wg.Wait()
// 		close(messages)
// 	}()

// 	// Collect all written messages
// 	var allMessages []string
// 	for msg := range messages {
// 		allMessages = append(allMessages, msg)
// 	}

// 	// Sort both expected and actual for comparison
// 	sort.Strings(allMessages)
// 	actualLines := strings.Split(strings.TrimSpace(buf1.String()), "\n")
// 	sort.Strings(actualLines)

// 	// Verify results
// 	assert.Equal(t, len(allMessages), len(actualLines))
// 	assert.Equal(t, strings.Join(allMessages, "\n"), strings.Join(actualLines, "\n"))
// 	assert.Equal(t, buf1.String(), buf2.String())
// }

func TestMultiWriter_NilWriter(t *testing.T) {
	_, err := NewMultiWriter(nil)
	assert.Error(t, err)

	buf := &bytes.Buffer{}
	writer := NewConsoleWriter(buf, ColorModeNever)
	multi, err := NewMultiWriter(writer)
	require.NoError(t, err)

	err = multi.AddWriter(nil)
	assert.Error(t, err)
}

func TestMultiWriter_DuplicateWriter(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewConsoleWriter(buf, ColorModeNever)

	multi, err := NewMultiWriter(writer)
	require.NoError(t, err)

	err = multi.AddWriter(writer)
	assert.Error(t, err)
}

func TestFileWriter_InvalidRotation(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "invalid_rotation.log")

	writer, err := NewFileWriter(FileConfig{
		Path:     logPath,
		MaxSize:  -1,
		Compress: false,
	})
	require.NoError(t, err)

	defer writer.Close()

	data := []byte("test data\n")
	_, err = writer.Write(data)
	assert.NoError(t, err)

	content, err := os.ReadFile(logPath)
	assert.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestFileWriter_WriteAfterClose(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "closed.log")

	writer, err := NewFileWriter(FileConfig{
		Path:     logPath,
		MaxSize:  1024,
		Compress: false,
	})
	require.NoError(t, err)

	writer.Close()

	_, err = writer.Write([]byte("test data\n"))
	assert.Error(t, err)
}
