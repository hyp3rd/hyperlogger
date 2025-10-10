package hyperlogger

//nolint:revive // Pointless to comment the colors.
const (
	// ANSI color codes for terminal output.

	// Regular colors.

	Black   = "\x1b[30m"
	Red     = "\x1b[31m"
	Green   = "\x1b[32m"
	Yellow  = "\x1b[33m"
	Blue    = "\x1b[34m"
	Magenta = "\x1b[35m"
	Cyan    = "\x1b[36m"
	White   = "\x1b[37m"

	// Bold colors.

	BoldBlack   = "\x1b[30;1m"
	BoldRed     = "\x1b[31;1m"
	BoldGreen   = "\x1b[32;1m"
	BoldYellow  = "\x1b[33;1m"
	BoldBlue    = "\x1b[34;1m"
	BoldMagenta = "\x1b[35;1m"
	BoldCyan    = "\x1b[36;1m"
	BoldWhite   = "\x1b[37;1m"

	// Reset resets the terminal's color settings.
	Reset = "\x1b[0m"
)

// DefaultLevelColors returns a map of log levels to their default ANSI color codes.
// The colors are chosen to provide good visibility and contrast for each log level.
func DefaultLevelColors() map[Level]string {
	return map[Level]string{
		TraceLevel: Magenta,
		DebugLevel: Blue,
		InfoLevel:  Green,
		WarnLevel:  Yellow,
		ErrorLevel: Red,
		FatalLevel: BoldRed,
	}
}

// ColorConfig holds color-related configuration for the logger.
// Enable enables colored output.
// ForceTTY forces colored output even when stdout is not a terminal.
// LevelColors maps log levels to their ANSI color codes.
type ColorConfig struct {
	// Enable enables colored output
	Enable bool
	// ForceTTY forces colored output even when stdout is not a terminal
	ForceTTY bool
	// LevelColors maps log levels to their ANSI color codes
	LevelColors map[Level]string
}

// DefaultColorConfig returns the default color configuration for the logger.
// The returned ColorConfig has Enable set to true, ForceTTY set to false,
// and LevelColors set to the DefaultLevelColors map.
func DefaultColorConfig() ColorConfig {
	return ColorConfig{
		Enable:      true,
		ForceTTY:    true,
		LevelColors: DefaultLevelColors(),
	}
}
