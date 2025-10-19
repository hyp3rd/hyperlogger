# Performance Improvement

Zero‑alloc hot path

Revisit WithFields/levels chaining: cache immutable field slices per adapter and reuse them via pooling (sync.Pool of []Field), so every .WithFields doesn’t allocate a new slice when depth is small. **(Adapter now pools log snapshots, avoids map-based merges, and pre-sizes for context extras; residual allocations come from extractor output.)**
Inline frequently used typed field helpers in adapter (e.g., WithError, WithField) to leverage stack allocation and avoid interface conversions. **(Adapter now handles WithField/WithError without map merges; consider context merge pooling next.)**
Optimize fmt.Sprintf usage inside logging: replace with pre-sized bytes.Buffer or strconv.Append* helpers for level/formatting to keep per-log allocations at ≤2. **(Caller formatting and primitive value formatting now use strconv; remaining fmt usage isolated to complex/default cases.)**
Encoder buffering

Introduce reusable encoder buffers per goroutine (e.g., a small sync.Pool keyed by enableJSON). Each log currently grabs a new bytes.Buffer; pooling reduces B/op and alloc count noticeably.
For JSON encoding, switch to json.Encoder alternatives or handcrafted append-based encoding to avoid temporary strings.
Async writer

Current bypass path copies payload (make([]byte, len(data))). Replace with buffer leasing via a pre-allocated slab or arena-like struct (still per-instance, not global) to reuse memory. **(AsyncWriter now borrows payload slices from a per-writer sync.Pool, reclaims them after write/flush/drop paths, and reuses the storage for Write and WriteCritical.)**
Drop handler currently copies into new slice; allow handler to signal “already copied” or provide a pre-allocated scratch to minimize duplicates. **(Advanced drop handlers can now retain pooled payloads via leases and release them when finished, avoiding duplicate copying.)**
Multi-writer path

io.MultiWriter (and our custom multi writer) adds allocations when wrapping simple buffers. Introduce a lightweight dualWriter for common two-sink cases to keep 0 alloc.
For WithAsyncMetricsHandler, current wrapper builds AsyncMetrics snapshots each time; consider reusing struct or passing pointer to reduce copying.
Benchmark-driven fine tuning

Add micro-benchmarks around WithFields, JSON encoding, and async bypass path to isolate allocation sources. Optimize until go test -bench BenchmarkAdapterLogging -benchmem shows ≤2 allocs for the “NoFields” case.
Run pprof heap profiles during benchmarks to confirm culprit allocations; use -alloc_space focus filters.
Compiler guidance

Ensure structs are stack-friendly: tag functions with //go:nosplit or //go:noinline only where it helps escape analysis (carefully measured).
Use small value types instead of interfaces when possible (e.g., replace any with concrete types inside inner loops).
