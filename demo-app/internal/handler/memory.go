package handler

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

type MemoryHandler struct {
	metrics             *otel.Metrics
	maxMemoryMB         int64
	mu                  sync.Mutex
	totalAllocatedBytes int64
}

func NewMemoryHandler(metrics *otel.Metrics, maxMemoryMB int64) *MemoryHandler {
	return &MemoryHandler{
		metrics:     metrics,
		maxMemoryMB: maxMemoryMB,
	}
}

func (h *MemoryHandler) AllocatedBytes() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.totalAllocatedBytes
}

func (h *MemoryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mbStr := r.URL.Query().Get("mb")
	mb := int64(256)
	if m, err := strconv.ParseInt(mbStr, 10, 64); err == nil && m > 0 {
		mb = m
	}

	durationStr := r.URL.Query().Get("duration")
	duration := 60 * time.Second
	if d, err := time.ParseDuration(durationStr); err == nil {
		duration = d
	}

	rampStr := r.URL.Query().Get("ramp")
	ramp := 15 * time.Second
	if d, err := time.ParseDuration(rampStr); err == nil {
		ramp = d
	}

	h.mu.Lock()
	if h.totalAllocatedBytes+mb*1024*1024 > h.maxMemoryMB*1024*1024 {
		h.mu.Unlock()
		http.Error(w, fmt.Sprintf("allocation would exceed MAX_MEMORY_MB (%d)", h.maxMemoryMB), http.StatusBadRequest)
		return
	}
	h.mu.Unlock()

	go h.allocateMemory(context.WithoutCancel(r.Context()), mb, duration, ramp)

	fmt.Fprintf(w, "Memory allocation started: mb=%d duration=%s ramp=%s\n", mb, duration, ramp)
}

func mmapAlloc(size int) ([]byte, error) {
	return syscall.Mmap(-1, 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
}

func touchPages(b []byte) {
	for i := 0; i < len(b); i += 4096 {
		b[i] = 1
	}
}

func (h *MemoryHandler) allocateMemory(ctx context.Context, targetMB int64, duration, ramp time.Duration) {
	totalDuration := duration + ramp
	startTime := time.Now()

	sustainPhase := duration - ramp
	if sustainPhase < 0 {
		sustainPhase = 0
	}

	var currentAlloc []byte
	var currentSize int

	defer func() {
		if currentAlloc != nil {
			syscall.Munmap(currentAlloc)
			h.mu.Lock()
			h.totalAllocatedBytes -= int64(currentSize)
			h.mu.Unlock()
		}
		h.metrics.MemoryAllocatedBytes.Record(ctx, 0)
		debug.FreeOSMemory()
	}()

	for time.Since(startTime) < totalDuration {
		elapsed := time.Since(startTime)

		var targetBytes int64
		switch {
		case elapsed < ramp:
			progress := float64(elapsed) / float64(ramp)
			targetBytes = int64(float64(targetMB) * progress * 1024 * 1024)
		case elapsed < ramp+sustainPhase:
			targetBytes = targetMB * 1024 * 1024
		case elapsed < ramp+sustainPhase+ramp:
			progress := float64(elapsed-ramp-sustainPhase) / float64(ramp)
			targetBytes = int64(float64(targetMB) * (1 - progress) * 1024 * 1024)
		default:
			return
		}

		newSize := int(targetBytes)
		if newSize < 0 {
			newSize = 0
		}

		if currentAlloc != nil && abs(newSize-currentSize) < 4096 {
			h.metrics.MemoryAllocatedBytes.Record(ctx, float64(currentSize))
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if currentAlloc != nil {
			syscall.Munmap(currentAlloc)
			h.mu.Lock()
			h.totalAllocatedBytes -= int64(currentSize)
			h.mu.Unlock()
			currentAlloc = nil
			currentSize = 0
		}

		if newSize > 0 {
			alloc, err := mmapAlloc(newSize)
			if err != nil {
				h.metrics.MemoryAllocatedBytes.Record(ctx, 0)
				return
			}
			touchPages(alloc)
			currentAlloc = alloc
			currentSize = newSize
			h.mu.Lock()
			h.totalAllocatedBytes += int64(newSize)
			h.mu.Unlock()
		}

		h.metrics.MemoryAllocatedBytes.Record(ctx, float64(currentSize))
		time.Sleep(100 * time.Millisecond)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
