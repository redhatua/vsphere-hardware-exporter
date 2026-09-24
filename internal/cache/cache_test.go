package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redhatua/vsphere-hardware-exporter/internal/inventory"
)

type fakeFetcher struct {
	fn    func(ctx context.Context) (*inventory.Snapshot, error)
	calls atomic.Int32
}

func (f *fakeFetcher) Fetch(ctx context.Context) (*inventory.Snapshot, error) {
	f.calls.Add(1)
	return f.fn(ctx)
}

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestFailureKeepsLastGoodSnapshot(t *testing.T) {
	fail := false
	f := &fakeFetcher{fn: func(context.Context) (*inventory.Snapshot, error) {
		if fail {
			return nil, errors.New("boom")
		}
		return &inventory.Snapshot{VCenter: "vc.example.com", Hosts: []inventory.Host{{Name: "h1"}}}, nil
	}}
	c := New(f, time.Hour, time.Second, quiet)

	if c.Ready() {
		t.Fatal("must not be ready before first refresh")
	}
	c.Refresh(context.Background())
	snap, st := c.Get()
	if !c.Ready() || !st.Up || len(snap.Hosts) != 1 || st.LastSuccess.IsZero() {
		t.Fatalf("after success: ready=%v state=%+v", c.Ready(), st)
	}
	ok := st.LastSuccess

	fail = true
	c.Refresh(context.Background())
	snap, st = c.Get()
	if st.Up {
		t.Error("up must be false after failure")
	}
	if snap == nil || len(snap.Hosts) != 1 {
		t.Error("last good snapshot must be kept")
	}
	if !st.LastSuccess.Equal(ok) {
		t.Error("last success timestamp must not advance on failure")
	}
	if !c.Ready() {
		t.Error("stays ready with stale data")
	}
}

func TestSlowFetchDoesNotBlockGet(t *testing.T) {
	release := make(chan struct{})
	f := &fakeFetcher{fn: func(ctx context.Context) (*inventory.Snapshot, error) {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil, errors.New("released")
	}}
	c := New(f, time.Hour, time.Minute, quiet)
	done := make(chan struct{})
	go func() { c.Refresh(context.Background()); close(done) }()

	got := make(chan struct{})
	go func() { c.Get(); close(got) }()
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("Get blocked behind a slow refresh")
	}
	close(release)
	<-done
}

func TestRefreshTimeout(t *testing.T) {
	f := &fakeFetcher{fn: func(ctx context.Context) (*inventory.Snapshot, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	c := New(f, time.Hour, 20*time.Millisecond, quiet)
	c.Refresh(context.Background())
	if _, st := c.Get(); st.Up {
		t.Error("timed-out refresh must not be up")
	}
}

func TestRunRefreshesPeriodically(t *testing.T) {
	f := &fakeFetcher{fn: func(context.Context) (*inventory.Snapshot, error) { return &inventory.Snapshot{}, nil }}
	c := New(f, 10*time.Millisecond, time.Second, quiet)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for f.calls.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("only %d refreshes", f.calls.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func TestRunRetriesSoonAfterFailure(t *testing.T) {
	var n atomic.Int32
	f := &fakeFetcher{fn: func(context.Context) (*inventory.Snapshot, error) {
		if n.Add(1) < 3 {
			return nil, errors.New("vcenter down")
		}
		return &inventory.Snapshot{}, nil
	}}
	// Regular interval is an hour; the retry delay must apply instead.
	c := New(f, time.Hour, time.Second, quiet)
	c.retry = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	deadline := time.After(2 * time.Second)
	for !c.Ready() {
		select {
		case <-deadline:
			t.Fatalf("not ready after %d attempts; failed refresh was not retried early", f.calls.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
}
