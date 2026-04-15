package loadgen

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/maxanderson95/k8s-autoscaling/demo-app/internal/otel"
)

type Generator struct {
	metrics       *otel.Metrics
	port          int
	activeWorkers atomic.Int64
}

func New(metrics *otel.Metrics, port int) *Generator {
	return &Generator{metrics: metrics, port: port}
}

func (g *Generator) ActiveWorkers() int64 {
	return g.activeWorkers.Load()
}

func (g *Generator) Start(ctx context.Context, targetWorkers int, duration, ramp time.Duration) {
	totalDuration := duration + ramp
	startTime := time.Now()

	sustainEnd := duration
	rampDownEnd := duration + ramp

	for time.Since(startTime) < totalDuration {
		elapsed := time.Since(startTime)

		var currentWorkers int
		switch {
		case elapsed < ramp:
			progress := float64(elapsed) / float64(ramp)
			currentWorkers = int(float64(targetWorkers) * progress)
			if currentWorkers < 1 {
				currentWorkers = 1
			}
		case elapsed < sustainEnd:
			currentWorkers = targetWorkers
		case elapsed < rampDownEnd:
			progress := float64(elapsed-sustainEnd) / float64(ramp)
			currentWorkers = int(float64(targetWorkers) * (1 - progress))
			if currentWorkers < 0 {
				currentWorkers = 0
			}
		default:
			return
		}

		active := int(g.activeWorkers.Load())
		diff := currentWorkers - active

		if diff > 0 {
			for i := 0; i < diff; i++ {
				g.activeWorkers.Add(1)
				go func() {
					defer g.activeWorkers.Add(-1)
					for {
						select {
						case <-ctx.Done():
							return
						default:
							url := fmt.Sprintf("http://localhost:%d/fibonacci?n=35", g.port)
							resp, err := http.Get(url)
							if err == nil {
								resp.Body.Close()
							}
						}
					}
				}()
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}
