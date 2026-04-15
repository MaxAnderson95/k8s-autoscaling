# Kubernetes Autoscaling — Hands-On Learning

A practical, hands-on repository for learning Kubernetes autoscaling by running real workloads against real clusters. No theoretical reading — each topic is exercised with a purpose-built test application and observed through actual metrics.

## Autoscaling Topics

This repo covers the three dimensions of Kubernetes autoscaling, plus the tools that make them more powerful:

| Topic | What It Does | Directory |
|---|---|---|
| **Horizontal Pod Autoscaler (HPA)** | Scales pod replicas based on CPU, memory, or custom metrics | `hpa/` |
| **Vertical Pod Autoscaler (VPA)** | Right-sizes pod resource requests based on actual usage | `vpa/` |
| **Cluster Autoscaler** | Adds/removes nodes when pods can't schedule or nodes are underutilized | `cluster-autoscaler/` |
| **KEDA** | Event-driven HPA — scales on queues, HTTP rates, cron schedules, and more | `keda/` |
| **Karpenter** | Fast, flexible node provisioning (alternative to Cluster Autoscaler) | `karpenter/` |

Each directory will contain manifests, walkthroughs, and observations from running against a real cluster.

## Test Application

`demo-app/` is a Go HTTP service designed specifically for triggering autoscaling behavior. Every endpoint has configurable **ramp-up and ramp-down** so load changes look realistic, not like a step function.

### Endpoints

| Endpoint | Method | What It Does | Key Parameters |
|---|---|---|---|
| `/cpu` | POST | Burns CPU cores with SHA256 hashing | `intensity` (goroutines), `duration`, `ramp` |
| `/memory` | POST | Allocates and holds RAM using mmap | `mb`, `duration`, `ramp` |
| `/load` | POST | Self-generates HTTP traffic against `/fibonacci` | `workers`, `duration`, `ramp` |
| `/fibonacci` | GET | Synchronous recursive Fibonacci — deterministic CPU cost per request | `n` (default 40) |
| `/queue` | POST | Sets a fake queue depth gauge (for KEDA) — decays 10%/sec | `depth`, `ramp` |
| `/status` | GET | JSON snapshot of active burns, allocations, queue depth, goroutines | — |
| `/healthz` | GET | Liveness/readiness probe | — |
| `/metrics` | GET | OpenTelemetry metrics in Prometheus format | — |

### How Ramp Works

All load-generating endpoints (`/cpu`, `/memory`, `/load`, `/queue`) accept a `ramp` parameter. Load increases linearly over the ramp period, sustains for the remaining duration, then ramps down linearly over another ramp period.

```
Total wall time = duration + ramp

     ramp          duration-ramp         ramp
   ┌──────┐────────────────────────┌──────┐
   │ ramp │       sustain           │ramp  │
   │  up  │                         │down  │
───┘      └─────────────────────────┘      └───
```

Example: `POST /cpu?intensity=4&duration=30s&ramp=10s`
- Over 10s: ramps from 1 to 4 goroutines
- From 10s–30s: sustains 4 goroutines
- From 30s–40s: ramps from 4 back down to 0

### Observability

Metrics are exposed via OpenTelemetry with two export paths:

1. **Prometheus** (`/metrics`) — always on, used by HPA custom metrics adapter and KEDA
2. **OTLP** — optional, configured via `OTEL_EXPORTER_OTLP_ENDPOINT` and `OTEL_EXPORTER_OTLP_PROTOCOL` env vars. Sends the same metrics to Dash0, Grafana, or any OTLP-compatible backend.

Set `OTEL_DISABLED=1` to disable all observability (useful for local testing without a backend).

### Memory Implementation

Memory allocation uses `mmap`/`munmap` instead of Go slices. This means RSS actually drops when memory is released — Go's garbage collector would normally hold onto freed pages, but `munmap` returns them to the OS immediately. This makes memory-based autoscaling exercises observable in `kubectl top` and `rss` metrics.

### Quick Start

```bash
# Build
cd demo-app && make build

# Run locally
PORT=8080 MAX_MEMORY_MB=1024 ./bin/demo-app

# Or: run with OTel disabled
OTEL_DISABLED=1 ./bin/demo-app

# Test it
curl http://localhost:8080/status
curl -X POST 'http://localhost:8080/cpu?intensity=2&duration=30s&ramp=10s'
curl -X POST 'http://localhost:8080/memory?mb=256&duration=60s&ramp=15s'
curl -X POST 'http://localhost:8080/load?workers=4&duration=120s&ramp=30s'
curl 'http://localhost:8080/fibonacci?n=40'
curl -X POST 'http://localhost:8080/queue?depth=500&ramp=20s'
curl http://localhost:8080/metrics
```

### Deploy to Kubernetes

```bash
# Install from GHCR (released version)
helm install demo-app oci://ghcr.io/maxanderson95/k8s-autoscaling/demo-app --version 0.0.2 -n auto-scaling-demo

# Or: install from local chart for development
helm install demo-app ./chart -n auto-scaling-demo

# Port-forward and test
kubectl port-forward svc/demo-app 8080:80 -n auto-scaling-demo -n auto-scaling-demo
curl http://localhost:8080/status
```

The Helm chart includes a Deployment (1 replica, 50m CPU / 64Mi memory requests), ClusterIP Service, and optional ServiceMonitor for Prometheus scraping.

## Repository Structure

```
.
├── demo-app/              # Test application + Helm chart
│   ├── cmd/server/         # Entrypoint
│   ├── internal/
│   │   ├── handler/        # HTTP handlers (cpu, memory, load, fibonacci, queue, status)
│   │   ├── loadgen/        # Self-load generation
│   │   └── otel/           # OpenTelemetry init (Prometheus + OTLP dual export)
│   ├── chart/              # Helm chart
│   ├── Dockerfile
│   └── Makefile
├── hpa/                    # (upcoming) HPA walkthroughs
├── vpa/                    # (upcoming) VPA walkthroughs
├── keda/                   # (upcoming) KEDA ScaledObject examples
├── cluster-autoscaler/    # (upcoming) Cluster Autoscaler exercises
└── karpenter/              # (upcoming) Karpenter provisioning examples
```