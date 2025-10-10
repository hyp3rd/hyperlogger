// Package constants provides application-wide constant values
// used throughout the logger system. These constants define
// environment names, configuration keys, and other fixed values
// to ensure consistency across the codebase.
package constants

import "time"

const (
	// NonProductionEnvironment is the environment name for non-production environments.
	NonProductionEnvironment = "development"
	// DefaultTimeout is the default timeout for asynchronous logging.
	DefaultTimeout = 5 * time.Second
)
