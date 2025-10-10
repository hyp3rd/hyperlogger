package constants

type (
	// OutputType represents the type of output.
	OutputType string
	// TimeFormat represents the format of time.
	TimeFormat string
)

const (
	// Output types.

	// LogOutputStdout represents the standard output stream.
	LogOutputStdout OutputType = "stdout"
	// LogOutputStderr represents the standard error stream.
	LogOutputStderr OutputType = "stderr"
	// LogOutputFile represents a file output.
	LogOutputFile OutputType = "file"

	// Time formats.

	// TimeFormatUnix represents the Unix timestamp format.
	TimeFormatUnix TimeFormat = "unix"
	// TimeFormatUnixMs represents the Unix timestamp format with milliseconds.
	TimeFormatUnixMs TimeFormat = "unix_ms"
	// TimeFormatRFC3339 represents the RFC3339 timestamp format.
	TimeFormatRFC3339 TimeFormat = "rfc3339"
	// TimeFormatRFC represents the RFC3339 timestamp format.
	TimeFormatRFC TimeFormat = "rfc"
	// TimeFormatDefault represents the default timestamp format.
	TimeFormatDefault TimeFormat = "default"
)

// IsValid returns true if the given OutputType is a valid output type, and false otherwise.
func (o OutputType) IsValid() bool {
	switch o {
	case LogOutputStdout, LogOutputStderr, LogOutputFile:
		return true
	default:
		return false
	}
}

// String returns the string representation of the OutputType.
func (o OutputType) String() string {
	return string(o)
}

// IsValid returns true if the given TimeFormat is a valid time format, and false otherwise.
func (t TimeFormat) IsValid() bool {
	switch t {
	case TimeFormatUnix, TimeFormatUnixMs, TimeFormatRFC3339, TimeFormatRFC, TimeFormatDefault:
		return true
	default:
		return false
	}
}

// String returns the string representation of the TimeFormat.
func (t TimeFormat) String() string {
	return string(t)
}
