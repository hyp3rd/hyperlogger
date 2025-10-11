package constants

import (
	"reflect"
	"testing"
)

func TestContextKeys(t *testing.T) {
	expected := []any{
		NamespaceKey{},
		ServiceKey{},
		EnvironmentKey{},
		ApplicationKey{},
		ComponentKey{},
		RequestKey{},
		UserKey{},
		SessionKey{},
		TraceKey{},
	}

	got := ContextKeys()
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("ContextKeys() = %#v, want %#v", got, expected)
	}
}

func TestContextKeyMap(t *testing.T) {
	expected := map[string]any{
		"namespace":   NamespaceKey{},
		"service":     ServiceKey{},
		"environment": EnvironmentKey{},
		"application": ApplicationKey{},
		"component":   ComponentKey{},
		"request":     RequestKey{},
		"user":        UserKey{},
		"session_id":  SessionKey{},
		"trace_id":    TraceKey{},
	}

	got := ContextKeyMap()
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("ContextKeyMap() = %#v, want %#v", got, expected)
	}
}
