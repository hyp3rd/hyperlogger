package constants

// Context keys for values that may be stored in a context.Context and extracted
// by the logger.

type (
	// NamespaceKey is a type for the namespace key to avoid collisions.
	NamespaceKey struct{}
	// ServiceKey is a type for the service key to avoid collisions.
	ServiceKey struct{}
	// EnvironmentKey is a type for the environment key to avoid collisions.
	EnvironmentKey struct{}
	// ApplicationKey is a type for the application key to avoid collisions.
	ApplicationKey struct{}
	// ComponentKey is a type for the component key to avoid collisions.
	ComponentKey struct{}
	// RequestKey is a type for the request key to avoid collisions.
	RequestKey struct{}
	// UserKey is a type for the user key to avoid collisions.
	UserKey struct{}
	// SessionKey is a type for the session key to avoid collisions.
	SessionKey struct{}
	// TraceKey is a type for the trace key to avoid collisions.
	TraceKey struct{}
)

// ContextKeys returns a slice of all defined context key types.
func ContextKeys() []any {
	return []any{
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
}

// ContextKeyMap returns a map of string representations to their corresponding context key types.
func ContextKeyMap() map[string]any {
	return map[string]any{
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
}
