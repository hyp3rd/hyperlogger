# Integrations Roadmap

This document captures the integration work planned for hyperlogger across alerting services, cloud providers, and popular Go web frameworks.

## Overview

The primary goals for these integrations are:

- Provide drop-in middleware for the most common Go web frameworks (Fiber v3, Chi, Gin) that seamlessly enrich logs with request metadata and handle panic recovery.
- Offer first-party alerting sinks (PagerDuty, OpsGenie, Slack, generic webhooks) that leverage the async writer to avoid blocking the main logging flow.
- Deliver cloud-specific exporters for AWS CloudWatch, Google Cloud Logging, and Azure Monitor, designed to preserve the zero-allocation hot path wherever possible.
- Ship reference configurations and usage guides so teams can adopt integrations without manual plumbing.

## Web Framework Middleware

### 1. Fiber v3

- [x] Middleware skeleton that extracts request ID, method, path, status, latency.
- [ ] Panic recovery hook that logs stack traces using hyperlogger's structured fields.
- [ ] Ensure compatibility with Fiber's context pooling (avoid storing pointers outside the request lifecycle).
- [ ] Unit tests covering success, error, and panic scenarios.

### 2. Chi

- [x] Middleware capturing route pattern, URL params, request/response metadata.
- [ ] Optional sampling or level overrides per route.
- [ ] Tests verifying middleware chaining and context propagation.

### 3. Gin

- [x] Update / introduce Gin middleware mirroring Fiber/Chi features.
- [ ] Panic recovery integration with Gin's `Recovery()` for structured logs.
- [ ] Compatibility tests with Gin's testing helpers.

## Alerting Hooks

- [ ] PagerDuty hook with configurable severity threshold and incident details.
- [ ] Opsgenie hook with routing key support.
- [ ] Slack webhook hook for formatted alerts (consider batching to reduce HTTP requests).
- [ ] Generic webhook hook that can deliver JSON payloads to arbitrary endpoints with retries.
- [ ] Documentation demonstrating how to register hooks via configuration.

## Cloud Provider Exporters

- [ ] AWS CloudWatch exporter:
        - Buffered, batched submission.
        - IAM credential integration (env, shared config, IAM roles).
        - Retry/backoff settings.
- [ ] Google Cloud Logging exporter:
        - Structured payloads using GCP logging API.
        - Authentication via ADC.
- [ ] Azure Monitor exporter:
        - Support for Data Collector API (Log Analytics workspace).
- [ ] Each exporter should include integration tests (behind build tags) and clear docs for enabling.

## Observability

- [ ] Extend AsyncMetrics to include integration-specific counters (e.g., alert hook failures, cloud exporter retries).
- [ ] Provide dashboards or sample queries for tracking integration health.

## Documentation

- [ ] Update README / docs site with usage examples for each integration.
- [ ] Publish reference configuration snippets (YAML/JSON) for enabling hooks/exporters via config loader.
- [ ] Guidance on testing integrations locally (e.g., using mock servers or sandbox credentials).

---

This PRD will evolve as integrations ship. Each milestone should track:

1. Implementation status.
1. Tests and benchmarks.
1. Documentation updates.

Please ensure new code paths maintain the performance guarantees established in PRD-PERF (≤2 allocations per log in the hot path and zero allocations for disabled integrations).
