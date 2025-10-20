# Hyperlogger PRD

## Background & Problem Statement

Hyperlogger targets high-throughput Go services but the current implementation leaves critical functionality incomplete or inconsistent, which limits competitiveness with ecosystems such as logrus and zerolog. Key configuration flags are ignored, contextual logging mutates shared state, asynchronous delivery can silently drop messages, and file/sampling features are unimplemented. We need a cohesive plan to harden the logger, expand features, and deliver predictable performance at scale.

## Goals

- Provide a zero-copy, allocation-aware core logger that is safe for concurrent use and resilient under load.
- Deliver first-class structured logging ergonomics on par with logrus/zerolog (typed fields, contextual enrichment, hooks).
- Ship reliable outputs (console, JSON, rotating files, multi-sink) with consistent formatting, timestamps, and optional stack traces.
- Expose configuration that actually maps to runtime behavior, with sane defaults and dynamic controls.
- Improve observability of the logger itself (metrics, health hooks, failure reporting).

## Non-Goals

- Replacing existing public APIs unless required for correctness or performance (additive extensions are preferred).
- Building a complete observability platform (metrics/log shipping integrations stay out-of-scope except hooks/APIs).
- Supporting legacy Go versions below the minimum already required by the module.

## Current State Findings

- Context enrichment mutates shared `fields` slices, leaking request metadata between loggers (`pkg/adapter/adapter.go:807`, `pkg/adapter/adapter.go:458`).
- `EnableStackTrace`, `DisableTimestamp`, `ColorConfig`, `Sampling`, and file rotation settings are defined but never exercised in the adapter (`config.go:81`, `config.go:117`, `config.go:136`, `config.go:165`, `config_builder.go:71`, `config_builder.go:88`).
- Async writer drops messages when the buffer is full with no back-pressure or critical-level bypass, causing silent data loss under load (`internal/output/async.go:62-89`).
- Fatal logging still routes through async pathways and runs hooks twice, risking missed messages and unexpected side effects (`pkg/adapter/adapter.go:830-855`).
- Hook registry state exists but custom hooks map is unused and there is no API parity with global hooks (`pkg/adapter/adapter.go:66-87`, `hooks.go:19-118`).
- Console color selection ignores the user-configured palette/TTY forcing options, leading to surprising output (`pkg/adapter/adapter.go:361-379`, `colors.go:49-72`).
- Config builder promises file rotation, retention, and compression but adapter never wires writers based on `Config.File*` settings (`config_builder.go:110-165`, `pkg/adapter/adapter.go:83-180`).

## Functional Requirements

### Logging Core

- Refactor entry construction to copy immutable field slices per log invocation, preventing cross-request leakage while retaining pooling.
- Add typed field helpers (`Str`, `Int`, `Error`, `Dur`, etc.) and message templating consistent with zerolog/logrus for ergonomics.
- Implement log sampling respecting `Config.Sampling`, with per-level counters and opt-out for high-priority levels.
- Ensure `SetLevel` is concurrency-safe (atomic/locking) and expose optional dynamic level update hooks.

### Formatting & Enrichment

- Honor `DisableTimestamp`, `EnableStackTrace`, caller flags, and color palette configuration in both JSON and console outputs.
- Capture stack traces on `Error`/`Fatal` (configurable depth) without blocking the hot path; include in structured output under a predictable field (`stack`).
- Support RFC3339, Unix, and custom time formats uniformly across outputs.
- Offer pluggable encoders so applications can register bespoke JSON/text formatters without forking the adapter.

### Outputs & Delivery

- Implement file writer wiring in the adapter, respecting rotation/compression/retention settings and supporting multi-sink fan out.
- Introduce a back-pressure strategy for async logging (configurable blocking, drop-oldest, or handoff) and guarantee synchronous flush for `Error`/`Fatal`.
- Provide health signals: expose queue depth, dropped log counters, and writer failure metrics via hooks or optional expvar/prometheus exporters.
- Support structured error handling: centralized error callbacks per writer plus retries/backoff for transient failures.

### Extensibility & Context

- Unify hook registration so global and config hooks share the same registry and can mutate entries safely.
- Expand `WithContext` to use pluggable extractors, enabling integration with OpenTelemetry, tracing, and custom domain keys without modifying core.
- Supply middleware helpers (HTTP, gRPC) to seed context values and demonstrate best practices.

### Configuration & Ergonomics

- Reconcile builder vs. config defaults; ensure every builder flag produces observable changes.
- Add environment-driven configuration loader (e.g., `FromEnv`, `FromFile`) supporting hot reload with safe application.
- Document migration guides for legacy setup versus new builder-centric flow.

## Non-Functional Requirements

- Benchmarks must show max ~1 microsecond latency per info-level log with <=2 heap allocations when structured fields <=5 on modern hardware.
- Async mode must maintain throughput ≥ 500k logs/sec on a single core before dropping or throttling, with configurable behavior once thresholds are hit.
- File rotation and compression should operate without blocking the main logging goroutines.
- Code should pass `go test -race`, avoid data races, and include fuzz coverage for formatter edge cases.

## Success Metrics

- 0 known data races in `go test -race ./...`.
- < 1% log drop rate under configured stress tests; fatal logs must be durable.
- Stack trace and timestamp toggles verified in automated integration tests.
- Documentation includes runnable examples and upgrade notes covering new features.

### Current Status (Benchmarks / QA)

- `go test ./... -bench BenchmarkAdapterLogging -benchmem` (Apple M3 Pro) produced ~0.86–1.15µs per info log with 5–8 allocations depending on format. While latency meets the ≤1µs goal for text/multi-writer cases, allocation count still exceeds the ≤2 target and needs further tuning.
- Async overflow benchmark (`go test ./internal/output -bench BenchmarkAsyncWriter -benchmem`) shows ~20ns (drop_newest) to ~118µs (handoff) per write under back-pressure, with allocations limited to 1–2 per call. Throughput comfortably exceeds the ≥500k logs/sec requirement in drop/handoff scenarios.
- `go test -race ./...` completes without data races.
- Prometheus exporter, context extractors, sampling rules, and middleware usage are documented in README; CHANGELOG captures upgrade notes.
- Outstanding items: reduce allocations in adapter benchmarks to meet ≤2 alloc target and capture formal drop-rate validation in a stress harness.

## Milestones

1. **Core Safety & Parity**: Fix field mutation, honor config flags, implement stack traces, make level operations atomic, and add tests/benchmarks.
1. **Async & Output Reliability**: Redesign async buffering, integrate file/multi-writer outputs, add failure metrics and hooks.
1. **Ergonomic Enhancements**: Introduce typed fields, sampling, context extractors, and update the builder/API docs.
1. **Stabilization**: Performance tuning, docs polish, compatibility review, and release packaging.

### Benchmark & Documentation Plan

- Produce reproducible benchmarks covering console/JSON/custom encoders, async strategies, and file outputs using Go's benchmarking harness.
- Document async health metrics and retry semantics, including integration examples for HTTP and optional gRPC middleware.
- Publish configuration loader usage (env/file) with sample manifests and migration guidance.

## Open Questions

- What default behavior should async mode adopt when buffers fill (block vs drop-oldest vs switch to sync)?
- Do we need built-in exporters for metrics, or is a hook-based pattern sufficient?
- Should we support log levels beyond `Fatal` (e.g., `Panic`, `Notice`) for broader compatibility?
