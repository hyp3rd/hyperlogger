package security

import (
	"math"
	"testing"
)

func TestSafeUint64FromInt(t *testing.T) {
	value, err := SafeUint64FromInt(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != 10 {
		t.Fatalf("expected 10, got %d", value)
	}

	if _, err = SafeUint64FromInt(-1); err == nil {
		t.Fatal("expected error for negative input")
	}
}

func TestSafeUint64FromInt64(t *testing.T) {
	const expectedValue = int64(42)

	value, err := SafeUint64FromInt64(expectedValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != uint64(expectedValue) {
		t.Fatalf("expected 42, got %d", value)
	}

	if _, err = SafeUint64FromInt64(-5); err == nil {
		t.Fatal("expected error for negative input")
	}
}

func TestSafeIntFromInt64(t *testing.T) {
	const expectedValue = int64(123)

	value, err := SafeIntFromInt64(expectedValue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != int(expectedValue) {
		t.Fatalf("expected 123, got %d", value)
	}

	maxInt := int64(^uint(0) >> 1)
	if maxInt < math.MaxInt64 {
		overflowCandidate := maxInt + 1
		if _, err = SafeIntFromInt64(overflowCandidate); err == nil {
			t.Fatal("expected overflow error")
		}
	}
}
