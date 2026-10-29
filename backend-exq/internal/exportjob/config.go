// Package exportjob holds the export job queue pieces of backend-exq.
package exportjob

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	minWorkers = 1
	maxWorkers = 8
	// maxJanitorInterval is the upper bound from Req 10.7.
	maxJanitorInterval = time.Minute
)

// WorkerConfig is the env-driven tuning for the export worker and janitor.
type WorkerConfig struct {
	Workers         int
	PollInterval    time.Duration
	JanitorInterval time.Duration
}

// LoadWorkerConfig reads EXPORT_WORKERS, EXPORT_POLL_INTERVAL and
// EXPORT_JANITOR_INTERVAL. Unset = default; an invalid value is an error so the
// service refuses to start (Req 10.7, 11.7).
func LoadWorkerConfig() (WorkerConfig, error) {
	cfg := WorkerConfig{Workers: 2, PollInterval: 2 * time.Second, JanitorInterval: 30 * time.Second}

	if v := os.Getenv("EXPORT_WORKERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < minWorkers || n > maxWorkers {
			return WorkerConfig{}, fmt.Errorf("EXPORT_WORKERS %q: must be an integer %d-%d", v, minWorkers, maxWorkers)
		}
		cfg.Workers = n
	}
	if v := os.Getenv("EXPORT_POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return WorkerConfig{}, fmt.Errorf("EXPORT_POLL_INTERVAL %q: must be a duration > 0", v)
		}
		cfg.PollInterval = d
	}
	if v := os.Getenv("EXPORT_JANITOR_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 || d > maxJanitorInterval {
			return WorkerConfig{}, fmt.Errorf("EXPORT_JANITOR_INTERVAL %q: must be a duration > 0 and <= %s", v, maxJanitorInterval)
		}
		cfg.JanitorInterval = d
	}
	return cfg, nil
}
