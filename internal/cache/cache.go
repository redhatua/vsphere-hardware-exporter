// Package cache holds the latest inventory snapshot and refreshes it in the background.
package cache

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/redhatua/vsphere-hardware-exporter/internal/collector"
	"github.com/redhatua/vsphere-hardware-exporter/internal/inventory"
)

// Fetcher reads a full inventory snapshot.
type Fetcher interface {
	Fetch(ctx context.Context) (*inventory.Snapshot, error)
}

// Cache serves the last good snapshot. Readers never block on vCenter.
type Cache struct {
	fetcher  Fetcher
	interval time.Duration
	timeout  time.Duration
	retry    time.Duration
	log      *slog.Logger

	mu    sync.RWMutex
	snap  *inventory.Snapshot
	state collector.State
}

// New creates a Cache. Each refresh is bounded by timeout.
func New(f Fetcher, interval, timeout time.Duration, log *slog.Logger) *Cache {
	return &Cache{fetcher: f, interval: interval, timeout: timeout, retry: retryInterval, log: log}
}

// Get implements collector.Source.
func (c *Cache) Get() (*inventory.Snapshot, collector.State) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap, c.state
}

// Ready reports whether at least one refresh has succeeded.
func (c *Cache) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap != nil
}

// retryInterval is how soon a failed refresh is retried (never later than the regular interval).
const retryInterval = time.Minute

// Refresh performs one fetch and reports whether it succeeded.
// On failure the previous snapshot is kept.
func (c *Cache) Refresh(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	snap, err := c.fetcher.Fetch(ctx)
	dur := time.Since(start)
	if err == nil && snap == nil {
		err = errors.New("fetcher returned no snapshot")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.state.Up = false
		c.log.Error("inventory refresh failed; serving last good data", "error", err, "duration", dur)
		return false
	}
	c.snap = snap
	c.state = collector.State{Up: true, LastSuccess: time.Now(), Duration: dur}
	c.log.Info("inventory refreshed", "hosts", len(snap.Hosts), "duration", dur)
	return true
}

func (c *Cache) delay(ok bool) time.Duration {
	if ok {
		return c.interval
	}
	return min(c.interval, c.retry)
}

// Run refreshes immediately and then every interval until ctx is cancelled.
// After a failed refresh it retries after retryInterval instead of waiting a full interval.
func (c *Cache) Run(ctx context.Context) {
	ok := c.Refresh(ctx)
	for {
		t := time.NewTimer(c.delay(ok))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			ok = c.Refresh(ctx)
		}
	}
}
