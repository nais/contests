// Package probe runs independent, periodic connectivity checks.
package probe

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Check verifies one platform service.
type Check struct {
	Name string
	Run  func(context.Context) error
}

// Runner records the outcome and completion time of each check.
type Runner struct {
	checks   []Check
	logger   *slog.Logger
	success  *prometheus.GaugeVec
	lastRun  *prometheus.GaugeVec
	duration *prometheus.GaugeVec
}

// New registers metrics for the configured checks.
func New(checks []Check, registry prometheus.Registerer, logger *slog.Logger) *Runner {
	r := &Runner{
		checks: checks,
		logger: logger,
		success: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "contests_check_success",
			Help: "Whether the last completed connectivity check succeeded.",
		}, []string{"check"}),
		lastRun: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "contests_check_last_run_timestamp_seconds",
			Help: "Unix timestamp of the last completed connectivity check.",
		}, []string{"check"}),
		duration: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "contests_check_duration_seconds",
			Help: "Duration of the last completed connectivity check.",
		}, []string{"check"}),
	}
	registry.MustRegister(r.success, r.lastRun, r.duration)
	for _, check := range checks {
		r.success.WithLabelValues(check.Name).Set(0)
		r.lastRun.WithLabelValues(check.Name).Set(0)
	}
	return r
}

// Run checks each service immediately and once per minute, until ctx is canceled.
// Each service has its own worker, so one outage does not block the other checks.
func (r *Runner) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, check := range r.checks {
		wg.Go(func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				if ctx.Err() != nil {
					return
				}
				r.runCheck(ctx, check)
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		})
	}
	wg.Wait()
}

func (r *Runner) runCheck(ctx context.Context, check Check) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	start := time.Now()
	err := check.Run(ctx)
	if err == nil {
		err = ctx.Err()
	}
	success := 1.0
	if err != nil {
		success = 0
		r.logger.With("check", check.Name, "err", err).Error("Connectivity check failed")
	}
	r.success.WithLabelValues(check.Name).Set(success)
	r.duration.WithLabelValues(check.Name).Set(time.Since(start).Seconds())
	r.lastRun.WithLabelValues(check.Name).SetToCurrentTime()
}
