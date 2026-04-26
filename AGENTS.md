# AGENTS.md

## Release Pipeline

- Tag format: `v*` (e.g. `v0.0.7`). Pushing a tag triggers `.github/workflows/release.yaml`.
- The workflow builds a **multi-arch** Docker image (`linux/amd64`, `linux/arm64`) and pushes it to GHCR.
- The Helm chart is published to GHCR as an OCI artifact. Image and chart have **separate GHCR repositories** to avoid tag collisions:
  - Container image: `ghcr.io/maxanderson95/k8s-autoscaling/demo-app`
  - Helm chart: `ghcr.io/maxanderson95/k8s-autoscaling/demo-app-chart`
- At release time, `yq` substitutes the version into `chart/Chart.yaml` (`.version`, `.appVersion`) and `chart/values.yaml` (`.image.tag`). The source `values.yaml` defaults to `0.0.0-dev` — this is a placeholder that only works when installing from the GHCR OCI chart, not from source.
- Use `yq`, never `sed`, for YAML mutations in the pipeline.
- Pin all GitHub Actions to full-length commit SHAs with a version comment.

## Demo App (`demo-app/`)

- Go HTTP service at `cmd/server/main.go`. Endpoints: `/cpu`, `/memory`, `/load`, `/fibonacci`, `/queue`, `/status`, `/healthz`, `/metrics`.
- All load endpoints support a `ramp` parameter for gradual increase/decrease — never instant spikes.
- OTel is optional at runtime: `OTEL_DISABLED=1` skips all instrumentation. Prometheus `/metrics` is always on.
- Memory allocation uses `mmap`/`munmap` (not Go slices) so RSS drops when freed.
- Go 1.25+ required (`prometheus/procfs` dependency). Dockerfile uses `golang:1.25` with `GOTOOLCHAIN=local`.
- Background goroutines must use `context.WithoutCancel(r.Context())` — `r.Context()` cancels after the HTTP response.

## Helm Chart (`demo-app/chart/`)

- Chart name: `demo-app-chart`. Distinct from container image name to prevent GHCR collisions.
- `values.yaml` has `nameOverride: demo-app` so deployed resources use `demo-app` as the app name, not the chart name.
- `serviceMonitor.enabled` defaults to `true`. Set `--set serviceMonitor.enabled=false` on clusters without Prometheus operator CRDs (e.g. kind).
- Chart version `0.0.0-dev` is a placeholder. `helm install ./chart` from source needs `--set image.tag=X.Y.Z`. `helm install oci://...` uses the released chart with correct tag already substituted.

## Demo Walkthroughs (`demos/`)

### Namespace and kubectl
- **Always** pass `-n auto-scaling-demo` to every `kubectl` command. Never use `kubectl config set-context --namespace`.
- Restoring the cluster context (`kubectl config use-context kind-autoscaling-learning`) is fine — that's cluster-level, not namespace-level.

### Load Testing
- Use **distributed load generators** inside the cluster that hit the **Service DNS**, not individual pods. This creates realistic load-balancing.
- The load generator pattern: a Deployment with `curlimages/curl` repeatedly hitting `/fibonacci?n=35` ~10 times/sec per pod.
- Hitting a single pod directly (e.g. `POST /cpu?intensity=4`) creates artificial concentrated load — all CPU on one pod. Don't use this for HPA demos.
- `/fibonacci?n=35` is the sweet spot for generating enough CPU to trigger HPA without overwhelming tiny clusters.
- Use a Deployment for the load generator (not a Job) so you can `kubectl scale` replicas up/down to control load.

### HPA Behavior
- HPA computes metrics from **Metrics Server** — `kubectl top pods` must work.
- Default scale-down `stabilizationWindowSeconds` is 300 (5 minutes). After load stops, expect pods to persist for ~5–8 minutes before scaling down.
- Scale-up is aggressive (default 0s stabilization, up to 100% per 15s). Scale-down is conservative (5min window).
- To speed up demos, patch the HPA: `kubectl patch hpa demo-app-cpu -n auto-scaling-demo --type merge -p '{"spec":{"behavior":{"scaleDown":{"stabilizationWindowSeconds":60}}}}'`.

## Local Development

- Use `kind` for local clusters: `kind create cluster --name autoscaling-learning`.
- Metrics Server must be installed and patched for kind: `kubectl patch deployment metrics-server -n kube-system ... --kubelet-insecure-tls`.
- The context sometimes resets to `localhost:8080` — check with `kubectl config current-context` and restore with `kubectl config use-context kind-autoscaling-learning`.
- Use `export KUBECONFIG=~/.kube/config` if multiple config files are in play.

## Commit Conventions

- Conventional commits: `feat:`, `fix:`, `chore:` prefixes.
- Branch naming: `type/description` in kebab-case.
- Never commit secrets or credentials.
- Never mention AI in commit messages.
- Never mention the company, its internal tooling, or proprietary setup in commits, code, config, or any files tracked in the repo — keep everything generic.
