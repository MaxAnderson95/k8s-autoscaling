# Basic HPA — Horizontal Pod Autoscaler

Walk through installing the demo app and observing Kubernetes HorizontalPodAutoscaler scale pods in response to CPU and memory load.

All commands assume you're in this directory (`demos/basic-hpa/`).

## Prerequisites

- A running Kubernetes cluster (any provider — kind, minikube, EKS, GKE, AKS, etc.)
- `kubectl` configured to talk to your cluster
- Metrics Server installed (required for CPU/memory-based HPA)

### Install Metrics Server

Metrics Server is not installed by default on most clusters. Check first:

```bash
kubectl top pods
```

If you get `error: Metrics API not available`, install it:

```bash
# kind / minikube / most dev clusters
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# If you get TLS certificate errors (common on kind), patch it:
kubectl patch deployment metrics-server -n kube-system --type json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
```

Verify it's working:

```bash
kubectl top nodes
```

You should see CPU and memory usage for your nodes.

## Step 1 — Create Namespace and Install the Demo App

Create the namespace (idempotent — safe to run again):

```bash
kubectl apply -f namespace.yaml
```

Install the demo app via Helm:

```bash
helm install demo-app oci://ghcr.io/maxanderson95/k8s-autoscaling/demo-app-chart -n auto-scaling-demo
```

Wait for the pod to be running:

```bash
kubectl get pods -n auto-scaling-demo -w
# NAME                        READY   STATUS    RESTARTS   AGE
# demo-app-xxxxxxxxxx-xxxxx   1/1     Running   0          30s
```

Port-forward so we can hit the endpoints:

```bash
kubectl port-forward svc/demo-app -n auto-scaling-demo 8080:80
```

In another terminal, verify it's up:

```bash
curl -s http://localhost:8080/status | jq .
# {
#   "uptime": "5s",
#   "goroutines": 5,
#   "heap_alloc_mb": 0,
#   "cpu_burns": 0,
#   "mem_allocated_bytes": 0,
#   "queue_depth": 0,
#   "load_workers": 0
# }
```

## Step 2 — Install HPA Resources

Apply the CPU-based HPA:

```bash
kubectl apply -f hpa-cpu.yaml -n auto-scaling-demo
```

This creates an HPA that will:
- Scale the deployment to 2–5 replicas when average CPU utilization exceeds 50%
- Scale back down to 1 replica when CPU drops below 50%

Check the HPA status:

```bash
kubectl get hpa -n auto-scaling-demo
# NAME             REFERENCE             TARGETS         MINPODS   MAXPODS   REPLICAS   AGE
# demo-app-cpu     Deployment/demo-app   <unknown>/50%   1         5         1          10s
```

`<unknown>` is normal for the first minute or so — Metrics Server needs time to collect data. Wait until you see actual percentages:

```bash
kubectl get hpa -n auto-scaling-demo -w
# NAME             REFERENCE             TARGETS   MINPODS   MAXPODS   REPLICAS   AGE
# demo-app-cpu     Deployment/demo-app   0%/50%     1         5         1          60s
```

## Step 3 — Trigger CPU Scale-Out

Hit the `/cpu` endpoint to ramp up CPU load:

```bash
curl -X POST 'http://localhost:8080/cpu?intensity=4&duration=120s&ramp=10s'
# CPU burn started: intensity=4 duration=120s ramp=10s
```

This gradually ramps from 1 to 4 CPU-burning goroutines over 10 seconds, sustains for 110 seconds, then ramps down over another 10 seconds.

Watch the HPA respond:

```bash
# Terminal 1: watch HPA decisions
kubectl get hpa -n auto-scaling-demo -w

# Terminal 2: watch pods scaling up
kubectl get pods -n auto-scaling-demo -w

# Terminal 3: watch resource usage
watch kubectl top pods -n auto-scaling-demo
```

You should see something like:

```
# HPA
NAME             REFERENCE             TARGETS    MINPODS   MAXPODS   REPLICAS
demo-app-cpu     Deployment/demo-app   185%/50%   1         5         3

# Pods
NAME                        READY   STATUS    RESTARTS   AGE
demo-app-xxxxxxxxxx-abcde   1/1     Running   0          45s
demo-app-xxxxxxxxxx-fghij   1/1     Running   0          30s
demo-app-xxxxxxxxxx-klmno   1/1     Running   0          15s
```

Key things to observe:
- CPU utilization jumps well above the 50% threshold
- HPA creates new pods (you'll see 2, then 3, possibly 4, depending on your node sizes)
- The `REPLICAS` column in `kubectl get hpa` increases
- Once the burn completes, utilization drops and pods eventually scale back down

## Step 4 — Observe Scale-Down

After the CPU burn ends (120 seconds), watch the pods scale back down. This takes a few minutes because HPA has a stabilization window (default 5 minutes from the last scale-up event, per `behavior.scaleDown.stabilizationWindowSeconds`).

```bash
# You can reduce the scale-down delay by editing the HPA:
kubectl patch hpa demo-app-cpu -n auto-scaling-demo --type merge -p '
{
  "spec": {
    "behavior": {
      "scaleDown": {
        "stabilizationWindowSeconds": 60
      }
    }
  }
}
'
```

## Step 5 — Memory-Based HPA

Clean up the CPU HPA first (they'll conflict since both target the same deployment):

```bash
kubectl delete hpa demo-app-cpu -n auto-scaling-demo
```

Apply the memory-based HPA:

```bash
kubectl apply -f hpa-memory.yaml -n auto-scaling-demo
```

Trigger memory allocation:

```bash
curl -X POST 'http://localhost:8080/memory?mb=256&duration=120s&ramp=10s'
# Memory allocation started: mb=256 duration=120s ramp=10s
```

Watch the memory utilization climb and pods scale out:

```bash
kubectl get hpa -n auto-scaling-demo -w
kubectl top pods -n auto-scaling-demo
```

The memory HPA triggers at 50% average memory utilization. With `limits: 512Mi` and allocating 256MB per pod, you should see it scale out quickly.

## Step 6 — Clean Up

```bash
# Remove HPA
kubectl delete hpa demo-app-cpu demo-app-memory -n auto-scaling-demo

# Remove the demo app
helm uninstall demo-app -n auto-scaling-demo

# Remove the namespace
kubectl delete namespace auto-scaling-demo
```

## What We Learned

| Concept | What We Observed |
|---|---|
| HPA reads metrics from Metrics Server | `kubectl top pods` must work for HPA to function |
| HPA is reactive, not predictive | It scales *after* load exceeds the threshold, not before |
| Scale-up is fast (~15s default) | New pods appear quickly once the threshold is breached |
| Scale-down has a cooldown window | Default 5-minute stabilization prevents flapping |
| `intensity` controls concurrent goroutines | 4 goroutines ≈ 4 CPU cores of load |
| `ramp` parameter makes load realistic | Gradual increase mimics production traffic patterns |
| Resource requests determine thresholds | HPA calculates % utilization against the pod's `resources.requests` |

## Next Steps

- **Custom Metrics HPA** — Use Prometheus Adapter + `/metrics` endpoint to scale on `demo_http_requests_in_flight` or `demo_cpu_burn_active`
- **KEDA** — Scale on `demo_fake_queue_depth` without needing Metrics Server for custom metrics
- **VPA** — Let Vertical Pod Autoscaler right-size your pod requests instead