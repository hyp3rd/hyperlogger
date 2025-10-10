package hyperlogger

import (
	"context"
	"reflect"
	"testing"

	"github.com/hyp3rd/ewrap"
)

// Mock types for testing.
type mockHook struct {
	levels    []Level
	onLogFunc func(entry *Entry) error
	executed  bool
}

func (m *mockHook) OnLog(entry *Entry) error {
	m.executed = true
	if m.onLogFunc != nil {
		return m.onLogFunc(entry)
	}

	return nil
}

func (m *mockHook) Levels() []Level {
	return m.levels
}

func TestRegisterHook(t *testing.T) {
	// Clean up hooks before and after test
	defer UnregisterAllHooks()

	UnregisterAllHooks()

	callCount := 0
	testHook := func(ctx context.Context, entry *Entry) error {
		callCount++

		return nil
	}

	// Register for specific level
	RegisterHook(InfoLevel, testHook)

	hooks.RLock()

	if len(hooks.funcs[InfoLevel]) != 1 {
		t.Errorf("Expected hook to be registered for InfoLevel, got %d hooks", len(hooks.funcs[InfoLevel]))
	}

	hooks.RUnlock()

	// Register nil hook (should be ignored)
	RegisterHook(InfoLevel, nil)

	hooks.RLock()

	if len(hooks.funcs[InfoLevel]) != 1 {
		t.Errorf("Expected nil hook to be ignored, got %d hooks", len(hooks.funcs[InfoLevel]))
	}

	hooks.RUnlock()

	// Test invalid level (should register for all levels)
	UnregisterAllHooks()
	RegisterHook(Level(100), testHook)

	hooks.RLock()

	for level := TraceLevel; level <= FatalLevel; level++ {
		if len(hooks.funcs[level]) != 1 {
			t.Errorf("Expected hook to be registered for level %d, got %d hooks", level, len(hooks.funcs[level]))
		}
	}

	hooks.RUnlock()
}

func TestRegisterGlobalHook(t *testing.T) {
	// Clean up hooks
	defer UnregisterAllHooks()

	UnregisterAllHooks()

	callCount := 0
	testHook := func(ctx context.Context, entry *Entry) error {
		callCount++

		return nil
	}

	RegisterGlobalHook(testHook)

	hooks.RLock()

	for level := TraceLevel; level <= FatalLevel; level++ {
		if len(hooks.funcs[level]) != 1 {
			t.Errorf("Expected hook to be registered for level %d, got %d hooks", level, len(hooks.funcs[level]))
		}
	}

	hooks.RUnlock()
}

func TestUnregisterHooks(t *testing.T) {
	// Clean up hooks
	defer UnregisterAllHooks()

	UnregisterAllHooks()

	testHook := func(ctx context.Context, entry *Entry) error {
		return nil
	}

	// Register for all levels
	RegisterGlobalHook(testHook)

	// Unregister for invalid level (should do nothing)
	UnregisterHooks(Level(100))

	hooks.RLock()

	for level := TraceLevel; level <= FatalLevel; level++ {
		if len(hooks.funcs[level]) != 1 {
			t.Errorf("Expected hooks to remain for level %d", level)
		}
	}

	hooks.RUnlock()

	// Unregister for specific level
	UnregisterHooks(InfoLevel)

	hooks.RLock()

	if _, exists := hooks.funcs[InfoLevel]; exists {
		t.Errorf("Expected hooks to be unregistered for InfoLevel")
	}

	hooks.RUnlock()
}

func TestUnregisterAllHooks(t *testing.T) {
	// Register hooks
	RegisterHook(InfoLevel, func(ctx context.Context, entry *Entry) error {
		return nil
	})

	// Unregister all hooks
	UnregisterAllHooks()

	hooks.RLock()

	if len(hooks.funcs) != 0 {
		t.Errorf("Expected all hooks to be unregistered, got %d", len(hooks.funcs))
	}

	hooks.RUnlock()
}

func TestHookRegistry(t *testing.T) {
	t.Run("NewHookRegistry", func(t *testing.T) {
		registry := NewHookRegistry()
		if registry == nil || registry.Hooks == nil {
			t.Fatal("Expected non-nil registry and hooks map")
		}
	})

	t.Run("AddHook", func(t *testing.T) {
		registry := NewHookRegistry()
		hook := &mockHook{levels: []Level{InfoLevel}}

		// Add new hook
		err := registry.AddHook("test-hook", hook)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Add duplicate hook (should error)
		err = registry.AddHook("test-hook", hook)
		if err == nil {
			t.Error("Expected error for duplicate hook name, got nil")
		}
	})

	t.Run("RemoveHook", func(t *testing.T) {
		registry := NewHookRegistry()
		hook := &mockHook{levels: []Level{InfoLevel}}

		// Add and remove hook
		_ = registry.AddHook("test-hook", hook)
		if !registry.RemoveHook("test-hook") {
			t.Error("Expected RemoveHook to return true")
		}

		// Remove non-existent hook
		if registry.RemoveHook("non-existent") {
			t.Error("Expected RemoveHook to return false")
		}
	})

	t.Run("GetHook", func(t *testing.T) {
		registry := NewHookRegistry()
		hook := &mockHook{levels: []Level{InfoLevel}}

		// Add hook
		_ = registry.AddHook("test-hook", hook)

		// Get existing hook
		got, exists := registry.GetHook("test-hook")
		if !exists {
			t.Error("Expected GetHook to find the hook")
		}

		if got != hook {
			t.Error("Expected GetHook to return the correct hook")
		}

		// Get non-existent hook
		_, exists = registry.GetHook("non-existent")
		if exists {
			t.Error("Expected GetHook to return false for non-existent hook")
		}
	})

	t.Run("GetHooksForLevel", func(t *testing.T) {
		registry := NewHookRegistry()
		infoHook := &mockHook{levels: []Level{InfoLevel}}
		debugHook := &mockHook{levels: []Level{DebugLevel}}
		multiHook := &mockHook{levels: []Level{InfoLevel, DebugLevel}}

		_ = registry.AddHook("info-hook", infoHook)
		_ = registry.AddHook("debug-hook", debugHook)
		_ = registry.AddHook("multi-hook", multiHook)

		// Get hooks for InfoLevel
		hooks := registry.GetHooksForLevel(InfoLevel)
		if len(hooks) != 2 {
			t.Errorf("Expected 2 hooks for InfoLevel, got %d", len(hooks))
		}

		// Get hooks for non-matching level
		hooks = registry.GetHooksForLevel(ErrorLevel)
		if len(hooks) != 0 {
			t.Errorf("Expected 0 hooks for ErrorLevel, got %d", len(hooks))
		}
	})

	t.Run("FireHooks", func(t *testing.T) {
		registry := NewHookRegistry()

		// Hook that succeeds
		successHook := &mockHook{
			levels: []Level{InfoLevel},
			onLogFunc: func(entry *Entry) error {
				return nil
			},
		}

		// Hook that fails
		failHook := &mockHook{
			levels: []Level{InfoLevel},
			onLogFunc: func(entry *Entry) error {
				return ewrap.New("hook failed")
			},
		}

		_ = registry.AddHook("success-hook", successHook)
		_ = registry.AddHook("fail-hook", failHook)

		entry := &Entry{Level: InfoLevel, Message: "test"}

		errors := registry.FireHooks(entry)

		if len(errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(errors))
		}

		if !successHook.executed || !failHook.executed {
			t.Error("Expected both hooks to be executed")
		}

		// Test with no matching hooks
		noMatchEntry := &Entry{Level: ErrorLevel, Message: "test"}

		errors = registry.FireHooks(noMatchEntry)
		if len(errors) != 0 {
			t.Errorf("Expected 0 errors for non-matching level, got %d", len(errors))
		}
	})
}

func TestStandardHook(t *testing.T) {
	t.Run("NewStandardHook", func(t *testing.T) {
		levels := []Level{InfoLevel, ErrorLevel}
		handler := func(entry *Entry) error { return nil }

		hook := NewStandardHook(levels, handler)

		if hook == nil {
			t.Fatal("Expected non-nil hook")
		}

		if !reflect.DeepEqual(hook.Levels(), levels) {
			t.Errorf("Expected levels %v, got %v", levels, hook.Levels())
		}
	})

	t.Run("OnLog", func(t *testing.T) {
		callCount := 0
		handler := func(entry *Entry) error {
			callCount++

			return ewrap.New("test error")
		}

		hook := NewStandardHook([]Level{InfoLevel}, handler)
		entry := &Entry{Level: InfoLevel, Message: "test"}

		err := hook.OnLog(entry)
		if err == nil || err.Error() != "test error" {
			t.Errorf("Expected error, got %v", err)
		}

		if callCount != 1 {
			t.Errorf("Expected handler to be called once, got %d", callCount)
		}

		// Test nil handler
		nilHook := NewStandardHook([]Level{InfoLevel}, nil)

		err = nilHook.OnLog(entry)
		if err != nil {
			t.Errorf("Expected nil error for nil handler, got %v", err)
		}
	})

	t.Run("Levels", func(t *testing.T) {
		levels := []Level{InfoLevel, ErrorLevel}
		hook := NewStandardHook(levels, nil)

		if !reflect.DeepEqual(hook.Levels(), levels) {
			t.Errorf("Expected levels %v, got %v", levels, hook.Levels())
		}
	})
}
