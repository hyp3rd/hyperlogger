package hyperlogger

import (
	"context"
	"slices"
	"sync"

	"github.com/hyp3rd/ewrap"
)

// Entry represents a log entry that's passed to hooks.
type Entry struct {
	// Level is the log level for this entry.
	Level Level
	// Message is the log message.
	Message string
	// Fields contains any structured data associated with this entry.
	Fields []Field
	// Raw provides access to the raw entry data for hook implementations
	// that need to access implementation-specific details.
	Raw any
}

// Hook is an interface that provides a way to hook into the logging process
// at various points.
type Hook interface {
	// OnLog is called when a log entry is being processed.
	OnLog(entry *Entry) error

	// Levels returns the log levels this hook should be triggered for.
	Levels() []Level
}

// LogHookFunc defines a hook function that executes during the logging process.
// Hooks are called after the log entry is created but before it's written to output.
// They can be used to modify log entries, perform additional actions based on log content,
// or integrate with external monitoring systems.
type LogHookFunc func(ctx context.Context, entry *Entry) error

// hooks maintains a registry of hook functions for each log level.
//
//nolint:gochecknoglobals
var hooks = struct {
	sync.RWMutex

	funcs map[Level][]LogHookFunc
}{
	funcs: make(map[Level][]LogHookFunc),
}

// RegisterHook adds a hook function for the specified log level.
// All hooks registered at a given level will be executed in registration order
// whenever a log message is generated at that level.
//
// If the level is invalid, the hook will be registered at all levels.
func RegisterHook(level Level, hookFunc LogHookFunc) {
	if hookFunc == nil {
		return
	}

	hooks.Lock()
	defer hooks.Unlock()

	if !level.IsValid() {
		// If level is invalid, register for all levels
		for l := TraceLevel; l <= FatalLevel; l++ {
			if _, exists := hooks.funcs[l]; !exists {
				hooks.funcs[l] = make([]LogHookFunc, 0, 2)
			}

			hooks.funcs[l] = append(hooks.funcs[l], hookFunc)
		}

		return
	}

	// Register hook for specified level
	if _, exists := hooks.funcs[level]; !exists {
		hooks.funcs[level] = make([]LogHookFunc, 0, 2)
	}

	hooks.funcs[level] = append(hooks.funcs[level], hookFunc)
}

// RegisterGlobalHook adds a hook function for all log levels.
func RegisterGlobalHook(hookFunc LogHookFunc) {
	for level := TraceLevel; level <= FatalLevel; level++ {
		RegisterHook(level, hookFunc)
	}
}

// UnregisterHooks removes all hooks for the specified level.
func UnregisterHooks(level Level) {
	if !level.IsValid() {
		return
	}

	hooks.Lock()
	defer hooks.Unlock()

	delete(hooks.funcs, level)
}

// UnregisterAllHooks removes all hooks for all levels.
func UnregisterAllHooks() {
	hooks.Lock()
	defer hooks.Unlock()

	hooks.funcs = make(map[Level][]LogHookFunc)
}

// LogHook is a container for LogHookFunc with associated metadata.
type LogHook struct {
	Name  string
	Func  LogHookFunc
	Level Level
}

// HookRegistry manages a collection of hooks and provides thread-safe access
// to them.
type HookRegistry struct {
	mu sync.RWMutex

	Hooks map[string]Hook
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		Hooks: make(map[string]Hook),
	}
}

// AddHook adds a named hook to the registry.
func (r *HookRegistry) AddHook(name string, hook Hook) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.Hooks[name]; exists {
		return ewrap.New("hook with already exists").WithMetadata("name", name)
	}

	r.Hooks[name] = hook

	return nil
}

// RemoveHook removes a hook by name.
func (r *HookRegistry) RemoveHook(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.Hooks[name]; !exists {
		return false
	}

	delete(r.Hooks, name)

	return true
}

// GetHook retrieves a hook by name.
func (r *HookRegistry) GetHook(name string) (Hook, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	hook, exists := r.Hooks[name]

	return hook, exists
}

// GetHooksForLevel returns all hooks that should trigger for a given level.
func (r *HookRegistry) GetHooksForLevel(level Level) []Hook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Hook

	for _, hook := range r.Hooks {
		if slices.Contains(hook.Levels(), level) {
			result = append(result, hook)
		}
	}

	return result
}

// FireHooks triggers all hooks for a given log entry
// It returns any errors encountered during hook execution.
func (r *HookRegistry) FireHooks(entry *Entry) []error {
	hooks := r.GetHooksForLevel(entry.Level)

	if len(hooks) == 0 {
		return nil
	}

	var errors []error

	for _, hook := range hooks {
		err := hook.OnLog(entry)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// StandardHook provides a simpler way to implement the Hook interface.
type StandardHook struct {
	// LevelList contains the levels this hook should trigger for
	LevelList []Level
	// LogHandler is called when a log entry is processed
	LogHandler func(entry *Entry) error
}

// NewStandardHook creates a new StandardHook with the given levels and handler.
func NewStandardHook(levels []Level, handler func(entry *Entry) error) *StandardHook {
	return &StandardHook{
		LevelList:  levels,
		LogHandler: handler,
	}
}

// OnLog implements Hook.OnLog.
func (h *StandardHook) OnLog(entry *Entry) error {
	if h.LogHandler != nil {
		return h.LogHandler(entry)
	}

	return nil
}

// Levels implements Hook.Levels.
func (h *StandardHook) Levels() []Level {
	return h.LevelList
}
