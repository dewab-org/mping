package concurrency

import (
	"context"
	"sync"
	"testing"
	"time"

	"mping/internal/ping"
	"mping/internal/state"
)

// stubBackend counts pings per host and always succeeds.
type stubBackend struct {
	mu    sync.Mutex
	calls map[string]int
}

func newStubBackend() *stubBackend {
	return &stubBackend{calls: map[string]int{}}
}

func (b *stubBackend) Ping(ctx context.Context, target ping.Target, timeout time.Duration) (ping.PingResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls[target.HostName]++
	return ping.PingResult{Success: true, RTT: time.Millisecond}, nil
}

func (b *stubBackend) count(host string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[host]
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func newTestState(t *testing.T, hosts ...string) *state.SharedState {
	t.Helper()
	st := state.NewSharedState(0)
	for _, h := range hosts {
		if err := st.AddHost(h, 20*time.Millisecond, 100*time.Millisecond); err != nil {
			t.Fatalf("AddHost %s: %v", h, err)
		}
	}
	return st
}

func TestWorkerPoolProcessesJob(t *testing.T) {
	st := newTestState(t, "h1")
	backend := newStubBackend()
	var notified sync.WaitGroup
	notified.Add(1)
	var once sync.Once
	pool := NewWorkerPool(context.Background(), backend, st, 2, 4, func() {
		once.Do(notified.Done)
	})
	defer pool.Close()

	if !pool.Submit(PingJob{HostKey: "h1", HostName: "h1", Protocol: "icmp", Timeout: 100 * time.Millisecond}) {
		t.Fatal("Submit returned false on open pool")
	}
	notified.Wait()

	if !waitFor(t, time.Second, func() bool { return st.Snapshot()[0].SuccessCount == 1 }) {
		t.Errorf("SuccessCount = %d, want 1", st.Snapshot()[0].SuccessCount)
	}
	if backend.count("h1") != 1 {
		t.Errorf("backend calls = %d, want 1", backend.count("h1"))
	}
}

func TestWorkerPoolSubmitAfterClose(t *testing.T) {
	st := newTestState(t)
	pool := NewWorkerPool(context.Background(), newStubBackend(), st, 1, 1, nil)
	pool.Close()
	if pool.Submit(PingJob{HostKey: "h1", HostName: "h1"}) {
		t.Error("Submit after Close should return false")
	}
}

func TestWorkerPoolCloseIsIdempotentAndStopsWorkers(t *testing.T) {
	st := newTestState(t)
	pool := NewWorkerPool(context.Background(), newStubBackend(), st, 4, 4, nil)
	pool.Close()
	pool.Close() // must not panic or deadlock
}

func TestSchedulerPingsImmediatelyAndRepeats(t *testing.T) {
	st := newTestState(t, "h1")
	backend := newStubBackend()
	pool := NewWorkerPool(context.Background(), backend, st, 2, 8, nil)
	defer pool.Close()
	group := NewSchedulerGroup(context.Background(), st, pool)
	defer group.StopAll()

	group.Start("h1")
	if !waitFor(t, time.Second, func() bool { return backend.count("h1") >= 2 }) {
		t.Fatalf("expected initial ping plus at least one scheduled ping, got %d", backend.count("h1"))
	}
}

func TestSchedulerStopHaltsPings(t *testing.T) {
	st := newTestState(t, "h1")
	backend := newStubBackend()
	pool := NewWorkerPool(context.Background(), backend, st, 2, 8, nil)
	defer pool.Close()
	group := NewSchedulerGroup(context.Background(), st, pool)
	defer group.StopAll()

	group.Start("h1")
	if !waitFor(t, time.Second, func() bool { return backend.count("h1") >= 1 }) {
		t.Fatal("scheduler never pinged")
	}
	group.Stop("h1")
	// One in-flight tick may still land after Stop; the count must then hold.
	time.Sleep(60 * time.Millisecond)
	settled := backend.count("h1")
	time.Sleep(120 * time.Millisecond)
	if got := backend.count("h1"); got != settled {
		t.Errorf("pings continued after Stop: %d -> %d", settled, got)
	}
}

func TestSchedulerExitsWhenHostDeleted(t *testing.T) {
	st := newTestState(t, "h1")
	backend := newStubBackend()
	pool := NewWorkerPool(context.Background(), backend, st, 2, 8, nil)
	defer pool.Close()
	group := NewSchedulerGroup(context.Background(), st, pool)
	defer group.StopAll()

	group.Start("h1")
	if !waitFor(t, time.Second, func() bool { return backend.count("h1") >= 1 }) {
		t.Fatal("scheduler never pinged")
	}
	st.DeleteHost("h1")
	time.Sleep(60 * time.Millisecond)
	settled := backend.count("h1")
	time.Sleep(120 * time.Millisecond)
	if got := backend.count("h1"); got != settled {
		t.Errorf("pings continued after host deletion: %d -> %d", settled, got)
	}
}

func TestSchedulerStartIsIdempotent(t *testing.T) {
	st := newTestState(t, "h1")
	backend := newStubBackend()
	pool := NewWorkerPool(context.Background(), backend, st, 2, 8, nil)
	defer pool.Close()
	group := NewSchedulerGroup(context.Background(), st, pool)
	defer group.StopAll()

	group.Start("h1")
	group.Start("h1") // second Start must not spawn a second scheduler
	// Only the first Start submits an immediate ping; a duplicate scheduler
	// would double the steady-state rate. Compare against ~3 intervals.
	time.Sleep(70 * time.Millisecond)
	got := backend.count("h1")
	if got > 5 {
		t.Errorf("ping count %d suggests duplicate schedulers", got)
	}
}
