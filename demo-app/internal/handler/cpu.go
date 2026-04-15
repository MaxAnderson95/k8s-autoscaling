package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

type CPUHandler struct {
	metrics *otel.Metrics
	mu      sync.Mutex
	burns   int64
}

func NewCPUHandler(metrics *otel.Metrics) *CPUHandler {
	return &CPUHandler{metrics: metrics}
}

func (h *CPUHandler) ActiveCount() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.burns
}

func (h *CPUHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	durationStr := r.URL.Query().Get("duration")
	duration := 30 * time.Second
	if d, err := time.ParseDuration(durationStr); err == nil {
		duration = d
	}

	rampStr := r.URL.Query().Get("ramp")
	ramp := 10 * time.Second
	if d, err := time.ParseDuration(rampStr); err == nil {
		ramp = d
	}

	intensityStr := r.URL.Query().Get("intensity")
	intensity := 2
	if i, err := strconv.Atoi(intensityStr); err == nil && i > 0 {
		intensity = i
	}

	h.mu.Lock()
	h.burns++
	h.mu.Unlock()

	go h.burnCPU(context.WithoutCancel(r.Context()), duration, ramp, intensity)

	fmt.Fprintf(w, "CPU burn started: intensity=%d duration=%s ramp=%s\n", intensity, duration, ramp)
}

func (h *CPUHandler) burnCPU(ctx context.Context, duration, ramp time.Duration, intensity int) {
	defer func() {
		h.mu.Lock()
		h.burns--
		h.mu.Unlock()
		h.recordActive(ctx)
	}()

	rampDownStart := duration
	rampDownEnd := duration + ramp
	totalDuration := rampDownEnd

	startTime := time.Now()

	for time.Since(startTime) < totalDuration {
		elapsed := time.Since(startTime)

		var targetGoroutines int
		switch {
		case elapsed < ramp:
			progress := float64(elapsed) / float64(ramp)
			targetGoroutines = int(float64(intensity) * progress)
			if targetGoroutines < 1 {
				targetGoroutines = 1
			}
		case elapsed < rampDownStart:
			targetGoroutines = intensity
		case elapsed < rampDownEnd:
			progress := float64(elapsed-rampDownStart) / float64(ramp)
			targetGoroutines = int(float64(intensity) * (1 - progress))
			if targetGoroutines < 0 {
				targetGoroutines = 0
			}
		default:
			return
		}

		var wg sync.WaitGroup
		for i := 0; i < targetGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data := []byte("autopilot")
				for j := 0; j < 10000; j++ {
					hash := sha256.Sum256(data)
					data = hash[:]
				}
			}()
		}
		wg.Wait()

		h.recordActive(ctx)
	}
}

func (h *CPUHandler) recordActive(ctx context.Context) {
	h.mu.Lock()
	active := h.burns
	h.mu.Unlock()
	h.metrics.CPUBurnActive.Record(ctx, int64(active))
}
