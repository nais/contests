package probe

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRecordsEachCheckOutcome(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		err  error
		want float64
	}{
		{name: "success", want: 1},
		{name: "failure", err: errors.New("unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			check := Check{Name: "test", Run: func(ctx context.Context) error {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("check has no deadline")
				}
				return tt.err
			}}
			runner := New([]Check{check}, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
			before := float64(time.Now().Unix())
			runner.runCheck(t.Context(), check)
			metrics, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			for _, metric := range metrics {
				value := metric.Metric[0].GetGauge().GetValue()
				switch metric.GetName() {
				case "contests_check_success":
					if value != tt.want {
						t.Errorf("success = %v, want %v", value, tt.want)
					}
				case "contests_check_last_run_timestamp_seconds":
					if value < before {
						t.Errorf("completion timestamp = %v, before check = %v", value, before)
					}
				}
			}
		})
	}
}

func TestChecksRunIndependentlyAndStopOnCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	blocked := make(chan struct{})
	completed := make(chan struct{})
	checks := []Check{
		{Name: "blocked", Run: func(ctx context.Context) error {
			close(blocked)
			<-ctx.Done()
			return ctx.Err()
		}},
		{Name: "healthy", Run: func(context.Context) error {
			close(completed)
			return nil
		}},
	}
	runner := New(checks, prometheus.NewRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	done := make(chan struct{})
	go func() {
		defer close(done)
		runner.Run(ctx)
	}()
	for _, ready := range []chan struct{}{blocked, completed} {
		select {
		case <-ready:
		case <-time.After(time.Second):
			cancel()
			t.Fatal("one check prevented the other from running")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after cancellation")
	}
}
