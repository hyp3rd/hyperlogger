package output

import (
	"io"
	"strconv"

	"github.com/hyp3rd/ewrap"
)

// Writer is an interface for log output writers.
type Writer interface {
	// Write writes the given bytes to the underlying output.
	Write(p []byte) (n int, err error)
	// Sync ensures that all data has been written.
	Sync() error
	// Close closes the writer and releases any resources.
	Close() error
}

type writerAdapter struct {
	writer io.Writer
}

// NewWriterAdapter wraps a basic io.Writer into a Writer interface implementation used by the output package.
func NewWriterAdapter(w io.Writer) Writer {
	return &writerAdapter{writer: w}
}

func (w *writerAdapter) Underlying() io.Writer {
	return w.writer
}

func (w *writerAdapter) Write(p []byte) (int, error) {
	bytes, err := w.writer.Write(p)
	if err != nil {
		return bytes, ewrap.Wrap(err, "failed to write to writer")
	}

	return bytes, nil
}

func (w *writerAdapter) Sync() error {
	if syncer, ok := w.writer.(interface{ Sync() error }); ok {
		return syncer.Sync()
	}

	return nil
}

func (w *writerAdapter) Close() error {
	if closer, ok := w.writer.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return ewrap.Wrap(err, "failed to close writer")
		}
	}

	return nil
}

// WriteResult holds the results of a write operation to a specific Writer.
// It includes the Writer instance, its name for diagnostics, bytes written, and any error encountered.
type WriteResult struct {
	Writer Writer // The writer instance
	Name   string // A descriptive name of the writer
	Bytes  int    // Number of bytes written
	Err    error  // Any error encountered during write
}

// ColorCode represents ANSI color codes for terminal output.
type ColorCode int

const (
	// ColorReset removes any color or style formatting.
	ColorReset ColorCode = 0
	// Color codes for foreground colors.

	// ColorBlack is the ANSI code for black text.
	ColorBlack ColorCode = iota + 29
	// ColorRed is the ANSI code for red text.
	ColorRed
	// ColorGreen is the ANSI code for green text.
	ColorGreen
	// ColorYellow is the ANSI code for yellow text.
	ColorYellow
	// ColorBlue is the ANSI code for blue text.
	ColorBlue
	// ColorMagenta is the ANSI code for magenta text.
	ColorMagenta
	// ColorCyan is the ANSI code for cyan text.
	ColorCyan
	// ColorWhite is the ANSI code for white text.
	ColorWhite
)

// Specific color codes for log levels.
const (
	// ColorCodeReset resets all color formatting.
	ColorCodeReset ColorCode = ColorReset
	// ColorCodeBlue is used for Debug level logs.
	ColorCodeBlue ColorCode = ColorBlue
	// ColorCodeGreen is used for Info level logs.
	ColorCodeGreen ColorCode = ColorGreen
	// ColorCodeYellow is used for Warning level logs.
	ColorCodeYellow ColorCode = ColorYellow
	// ColorCodeRed is used for Error level logs.
	ColorCodeRed ColorCode = ColorRed
	// ColorCodeMagenta is used for Trace level logs.
	ColorCodeMagenta ColorCode = ColorMagenta
	// ColorCodeRedBold is a combination of Red color with Bold style.
	ColorCodeRedBold ColorCode = ColorRed // Bold style is applied separately.
)

// Style represents text style formatting codes for terminal output.
type Style int

const (
	// StyleBold enables bold text.
	StyleBold Style = 1
	// StyleDim enables dimmed text.
	StyleDim Style = 2
	// StyleItalic enables italic text.
	StyleItalic Style = 3
	// StyleUnderline enables underlined text.
	StyleUnderline Style = 4
	// StyleNormal resets all formatting.
	StyleNormal Style = 22 // Reset to normal style
)

// ColorWithStyle applies a style to a color code.
func ColorWithStyle(color ColorCode, style Style) string {
	return strconv.Itoa(int(style)) + ";" + strconv.Itoa(int(color))
}

// Start returns the ANSI escape sequence to start this color.
func (c ColorCode) Start() string {
	if c == ColorReset {
		return "\033[0m"
	}

	return "\033[" + strconv.Itoa(int(c)) + "m"
}

// End returns the ANSI escape sequence to reset to default color.
func (ColorCode) End() string {
	return "\033[0m"
}

// Wrap wraps the given string with this color code.
func (c ColorCode) Wrap(s string) string {
	if c == ColorReset {
		return s
	}

	return c.Start() + s + c.End()
}

// WithStyle returns a new color string with the given style applied.
func (c ColorCode) WithStyle(style Style) string {
	if c == ColorReset {
		return "\033[" + strconv.Itoa(int(style)) + "m"
	}

	return "\033[" + strconv.Itoa(int(style)) + ";" + strconv.Itoa(int(c)) + "m"
}
