package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

type StatusResponse struct {
	Uptime       string `json:"uptime"`
	Goroutines   int    `json:"goroutines"`
	HeapAllocMB  int    `json:"heap_alloc_mb"`
	CPUBurns     int64  `json:"cpu_burns"`
	MemAllocated int64  `json:"mem_allocated_bytes"`
	QueueDepth   int64  `json:"queue_depth"`
	LoadWorkers  int64  `json:"load_workers"`
}

type StatusHandler struct {
	startTime time.Time
	cpu       *CPUHandler
	memory    *MemoryHandler
	queue     *QueueHandler
	load      *LoadHandler
}

func NewStatusHandler(cpu *CPUHandler, memory *MemoryHandler, queue *QueueHandler, load *LoadHandler) *StatusHandler {
	return &StatusHandler{
		startTime: time.Now(),
		cpu:       cpu,
		memory:    memory,
		queue:     queue,
		load:      load,
	}
}

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := StatusResponse{
		Uptime:       time.Since(h.startTime).Round(time.Second).String(),
		Goroutines:   runtime.NumGoroutine(),
		HeapAllocMB:  int(m.HeapAlloc / 1024 / 1024),
		CPUBurns:     h.cpu.ActiveCount(),
		MemAllocated: h.memory.AllocatedBytes(),
		QueueDepth:   h.queue.Depth(),
		LoadWorkers:  h.load.ActiveWorkers(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
