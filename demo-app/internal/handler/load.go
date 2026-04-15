package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/loadgen"
	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

type LoadHandler struct {
	metrics    *otel.Metrics
	port       int
	mu         sync.Mutex
	generators []*loadgen.Generator
}

func NewLoadHandler(metrics *otel.Metrics, port int) *LoadHandler {
	return &LoadHandler{metrics: metrics, port: port}
}

func (h *LoadHandler) ActiveWorkers() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	var total int64
	for _, g := range h.generators {
		total += g.ActiveWorkers()
	}
	return total
}

func (h *LoadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workersStr := r.URL.Query().Get("workers")
	workers := 4
	if w, err := strconv.Atoi(workersStr); err == nil && w > 0 {
		workers = w
	}

	durationStr := r.URL.Query().Get("duration")
	duration := 120 * time.Second
	if d, err := time.ParseDuration(durationStr); err == nil {
		duration = d
	}

	rampStr := r.URL.Query().Get("ramp")
	ramp := 30 * time.Second
	if d, err := time.ParseDuration(rampStr); err == nil {
		ramp = d
	}

	gen := loadgen.New(h.metrics, h.port)
	h.mu.Lock()
	h.generators = append(h.generators, gen)
	h.mu.Unlock()

	go gen.Start(context.WithoutCancel(r.Context()), workers, duration, ramp)

	fmt.Fprintf(w, "Load generation started: workers=%d duration=%s ramp=%s\n", workers, duration, ramp)
}
