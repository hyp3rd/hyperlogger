package security

import "github.com/hyp3rd/ewrap"

// ErrNegativeValue is returned when attempting to convert a negative number to an unsigned type.
var ErrNegativeValue = ewrap.New("negative values cannot be converted to unsigned integers")

// SafeUint64FromInt converts an int to uint64 while guarding against negative values and overflow.
func SafeUint64FromInt(value int) (uint64, error) {
	if value < 0 {
		return 0, ErrNegativeValue
	}

	return uint64(value), nil
}

// SafeUint64FromInt64 converts an int64 to uint64 while guarding against negative values.
func SafeUint64FromInt64(value int64) (uint64, error) {
	if value < 0 {
		return 0, ErrNegativeValue
	}

	return uint64(value), nil
}

// SafeIntFromInt64 converts an int64 to int, ensuring the value fits in the target type.
func SafeIntFromInt64(value int64) (int, error) {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1

	if value > maxInt || value < minInt {
		return 0, ewrap.New("int64 value overflows int")
	}

	return int(value), nil
}
