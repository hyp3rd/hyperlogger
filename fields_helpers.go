package hyperlogger

import (
	"time"
)

// Str creates a Field with a string value.
func Str(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a Field with a boolean value.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Int creates a Field with an int value.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates a Field with an int64 value.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Uint64 creates a Field with an uint64 value.
func Uint64(key string, value uint64) Field {
	return Field{Key: key, Value: value}
}

// Float64 creates a Field with a float64 value.
func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

// Duration creates a Field with a time.Duration value.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time creates a Field with a time.Time value.
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Error creates a Field from an error. Nil errors are ignored.
func Error(key string, err error) Field {
	var val any
	if err != nil {
		val = err
	}

	return Field{Key: key, Value: val}
}

// Any creates a Field with an arbitrary value.
func Any(key string, value any) Field {
	return Field{Key: key, Value: value}
}
