package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/handler"
	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

func main() {
	port, _ := strconv.Atoi(envOrDefault("PORT", "8080"))
	maxMemoryMB, _ := strconv.ParseInt(envOrDefault("MAX_MEMORY_MB", "1024"), 10, 64)

	ctx := context.Background()

	result, err := otel.Init(ctx, "demo-app", "0.1.0")
	if err != nil {
		log.Fatalf("failed to initialize OTel: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := result.Shutdown(shutdownCtx); err != nil {
			log.Printf("error shutting down OTel provider: %v", err)
		}
	}()

	metrics := result.Metrics

	mux := http.NewServeMux()
	cpuHandler := handler.NewCPUHandler(metrics)
	memHandler := handler.NewMemoryHandler(metrics, maxMemoryMB)
	loadHandler := handler.NewLoadHandler(metrics, port)
	queueHandler := handler.NewQueueHandler(metrics)

	mux.Handle("/cpu", cpuHandler)
	mux.Handle("/memory", memHandler)
	mux.Handle("/load", loadHandler)
	mux.HandleFunc("/fibonacci", handler.Fibonacci)
	mux.Handle("/queue", queueHandler)
	mux.HandleFunc("/healthz", handler.Healthz)
	mux.Handle("/status", handler.NewStatusHandler(cpuHandler, memHandler, queueHandler, loadHandler))
	mux.Handle("/metrics", result.PromHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("starting server on :%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
	log.Println("server exited")
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %v", r.Method, r.URL.Path, r.URL.RawQuery, time.Since(start).Round(time.Millisecond))
	})
}
