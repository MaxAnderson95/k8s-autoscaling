package otel

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Metrics struct {
	HTTPRequestsTotal    metric.Int64Counter
	HTTPRequestsInFlight metric.Int64UpDownCounter
	FakeQueueDepth       metric.Float64Gauge
	MemoryAllocatedBytes metric.Float64Gauge
	CPUBurnActive        metric.Int64Gauge
	Meter                metric.Meter
}

type OTelShutdown func(context.Context) error

type InitResult struct {
	Provider    metric.MeterProvider
	Metrics     *Metrics
	PromHandler http.Handler
	Shutdown    OTelShutdown
}

func Init(ctx context.Context, serviceName, serviceVersion string) (*InitResult, error) {
	if os.Getenv("OTEL_DISABLED") != "" {
		meter := noop.NewMeterProvider().Meter(serviceName)
		m, err := newMetrics(meter)
		if err != nil {
			return nil, fmt.Errorf("creating noop metrics: %w", err)
		}
		return &InitResult{
			Provider:    noop.NewMeterProvider(),
			Metrics:     m,
			PromHandler: http.HandlerFunc(otelDisabled),
			Shutdown:    func(ctx context.Context) error { return nil },
		}, nil
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	var opts []sdkmetric.Option
	opts = append(opts, sdkmetric.WithResource(res))
	opts = append(opts, sdkmetric.WithReader(promExporter))

	if endpoint := envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""); endpoint != "" {
		otlpReader, err := createOTLPReader(ctx, endpoint)
		if err != nil {
			log.Printf("warning: OTLP exporter not configured: %v", err)
		} else {
			opts = append(opts, sdkmetric.WithReader(otlpReader))
		}
	}

	provider := sdkmetric.NewMeterProvider(opts...)
	meter := provider.Meter(serviceName)

	m, err := newMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("creating metrics: %w", err)
	}

	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}

	return &InitResult{
		Provider:    provider,
		Metrics:     m,
		PromHandler: promhttp.Handler(),
		Shutdown:    shutdown,
	}, nil
}

func newMetrics(meter metric.Meter) (*Metrics, error) {
	m := &Metrics{Meter: meter}

	var err error

	m.HTTPRequestsTotal, err = meter.Int64Counter(
		"demo_http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http_requests_total: %w", err)
	}

	m.HTTPRequestsInFlight, err = meter.Int64UpDownCounter(
		"demo_http_requests_in_flight",
		metric.WithDescription("Number of HTTP requests currently in flight"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating http_requests_in_flight: %w", err)
	}

	m.FakeQueueDepth, err = meter.Float64Gauge(
		"demo_fake_queue_depth",
		metric.WithDescription("Simulated queue depth for KEDA testing"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating fake_queue_depth: %w", err)
	}

	m.MemoryAllocatedBytes, err = meter.Float64Gauge(
		"demo_memory_allocated_bytes",
		metric.WithDescription("Bytes intentionally allocated via /memory endpoint"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating memory_allocated_bytes: %w", err)
	}

	m.CPUBurnActive, err = meter.Int64Gauge(
		"demo_cpu_burn_active",
		metric.WithDescription("Number of active CPU burn goroutines"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating cpu_burn_active: %w", err)
	}

	return m, nil
}

func createOTLPReader(ctx context.Context, endpoint string) (sdkmetric.Reader, error) {
	protocol := envOrDefault("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	switch protocol {
	case "grpc":
		exporter, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(endpoint),
		)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP gRPC exporter: %w", err)
		}
		return sdkmetric.NewPeriodicReader(exporter), nil
	case "http":
		exporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(endpoint),
		)
		if err != nil {
			return nil, fmt.Errorf("creating OTLP HTTP exporter: %w", err)
		}
		return sdkmetric.NewPeriodicReader(exporter), nil
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol: %s", protocol)
	}
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func otelDisabled(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OTel disabled (set OTEL_DISABLED= to enable)\n"))
}
