package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

type QueueHandler struct {
	metrics *otel.Metrics
	mu      sync.Mutex
	depth   atomic.Int64
	cancel  context.CancelFunc
}

func NewQueueHandler(metrics *otel.Metrics) *QueueHandler {
	return &QueueHandler{metrics: metrics}
}

func (h *QueueHandler) Depth() int64 {
	return h.depth.Load()
}

func (h *QueueHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	depthStr := r.URL.Query().Get("depth")
	depth := int64(100)
	if d, err := strconv.ParseInt(depthStr, 10, 64); err == nil {
		depth = d
	}

	rampStr := r.URL.Query().Get("ramp")
	ramp := 10 * time.Second
	if d, err := time.ParseDuration(rampStr); err == nil {
		ramp = d
	}

	if depth == 0 {
		if h.cancel != nil {
			h.cancel()
		}
		h.depth.Store(0)
		h.metrics.FakeQueueDepth.Record(r.Context(), 0)
		fmt.Fprintln(w, "Queue drained")
		return
	}

	if h.cancel != nil {
		h.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel

	go h.rampQueue(ctx, depth, ramp)

	fmt.Fprintf(w, "Queue depth ramping to %d over %s\n", depth, ramp)
}

func (h *QueueHandler) rampQueue(ctx context.Context, targetDepth int64, ramp time.Duration) {
	startDepth := h.depth.Load()
	steps := int(ramp / (100 * time.Millisecond))
	if steps < 1 {
		steps = 1
	}

	stepDuration := ramp / time.Duration(steps)

	for i := 0; i <= steps; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress := float64(i) / float64(steps)
		currentDepth := int64(float64(startDepth) + float64(targetDepth-startDepth)*progress)
		h.depth.Store(currentDepth)
		h.metrics.FakeQueueDepth.Record(ctx, float64(currentDepth))
		time.Sleep(stepDuration)
	}

	h.decayLoop(ctx)
}

func (h *QueueHandler) decayLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := h.depth.Load()
			if current <= 0 {
				h.depth.Store(0)
				h.metrics.FakeQueueDepth.Record(ctx, 0)
				return
			}
			newDepth := int64(float64(current) * 0.9)
			h.depth.Store(newDepth)
			h.metrics.FakeQueueDepth.Record(ctx, float64(newDepth))
		}
	}
}
