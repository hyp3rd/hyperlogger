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

// globalHookRegistry stores hook registrations shared across all loggers.
//
//nolint:gochecknoglobals
var globalHookRegistry = NewHookRegistry()

// RegisterHook adds a hook function for the specified log level.
// All hooks registered at a given level will be executed in registration order
// whenever a log message is generated at that level.
//
// If the level is invalid, the hook will be registered at all levels.
func RegisterHook(level Level, hookFunc LogHookFunc) {
	if hookFunc == nil {
		return
	}

	globalHookRegistry.AddFunc(level, hookFunc)
}

// RegisterGlobalHook adds a hook function for all log levels.
func RegisterGlobalHook(hookFunc LogHookFunc) {
	for level := TraceLevel; level <= FatalLevel; level++ {
		globalHookRegistry.AddFunc(level, hookFunc)
	}
}

// UnregisterHooks removes all hooks for the specified level.
func UnregisterHooks(level Level) {
	globalHookRegistry.RemoveFuncs(level)
}

// UnregisterAllHooks removes all hooks for all levels.
func UnregisterAllHooks() {
	globalHookRegistry.ResetFuncs()
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
	funcs map[Level][]LogHookFunc
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		Hooks: make(map[string]Hook),
		funcs: make(map[Level][]LogHookFunc),
	}
}

// FireRegisteredHooks executes the global LogHookFuncs registered via RegisterHook/RegisterGlobalHook
// and returns any errors they produced.
func FireRegisteredHooks(ctx context.Context, entry *Entry) []error {
	return globalHookRegistry.Dispatch(ctx, entry)
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

// AddFunc registers a LogHookFunc for the specified level.
// If level is invalid, the hook is registered for all levels.
func (r *HookRegistry) AddFunc(level Level, hookFunc LogHookFunc) {
	if hookFunc == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if !level.IsValid() {
		for l := TraceLevel; l <= FatalLevel; l++ {
			r.funcs[l] = append(r.funcs[l], hookFunc)
		}

		return
	}

	r.funcs[level] = append(r.funcs[level], hookFunc)
}

// RemoveFuncs clears all function hooks for the provided level.
func (r *HookRegistry) RemoveFuncs(level Level) {
	if !level.IsValid() {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.funcs, level)
}

// ResetFuncs removes all registered function hooks.
func (r *HookRegistry) ResetFuncs() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.funcs = make(map[Level][]LogHookFunc)
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

// FireHooks triggers all hooks for a given log entry using a background context.
// Deprecated: use Dispatch to control context propagation.
func (r *HookRegistry) FireHooks(entry *Entry) []error {
	return r.Dispatch(context.Background(), entry)
}

// Dispatch triggers all hooks for a given log entry with context propagation.
// It returns any errors encountered during hook execution.
func (r *HookRegistry) Dispatch(ctx context.Context, entry *Entry) []error {
	if entry == nil {
		return nil
	}

	var errors []error

	for _, hook := range r.GetHooksForLevel(entry.Level) {
		err := hook.OnLog(entry)
		if err != nil {
			errors = append(errors, err)
		}
	}

	for _, fn := range r.getFuncsForLevel(entry.Level) {
		if fn == nil {
			continue
		}

		err := fn(ctx, entry)
		if err != nil {
			errors = append(errors, ewrap.Wrap(err, "hook execution failed"))
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

func (r *HookRegistry) getFuncsForLevel(level Level) []LogHookFunc {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if level.IsValid() {
		return append([]LogHookFunc(nil), r.funcs[level]...)
	}

	var funcs []LogHookFunc

	for l := TraceLevel; l <= FatalLevel; l++ {
		funcs = append(funcs, r.funcs[l]...)
	}

	return funcs
}
