package concurrency

import (
	"context"
	"sync"
	"time"

	"mping/internal/ping"
	"mping/internal/state"
)

type PingJob struct {
	HostKey  string
	HostName string
	Timeout  time.Duration
}

type WorkerPool struct {
	ctx     context.Context
	cancel  context.CancelFunc
	backend ping.PingBackend
	state   *state.SharedState
	jobs    chan PingJob
	wg      sync.WaitGroup
	notify  func()
}

func NewWorkerPool(ctx context.Context, backend ping.PingBackend, st *state.SharedState, workers int, queueCap int, notify func()) *WorkerPool {
	ctx, cancel := context.WithCancel(ctx)
	p := &WorkerPool{
		ctx:     ctx,
		cancel:  cancel,
		backend: backend,
		state:   st,
		jobs:    make(chan PingJob, queueCap),
		notify:  notify,
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job := <-p.jobs:
			p.runJob(job)
		}
	}
}

func (p *WorkerPool) runJob(job PingJob) {
	ctx, cancel := context.WithTimeout(p.ctx, job.Timeout)
	defer cancel()
	res, err := p.backend.Ping(ctx, job.HostName, job.Timeout)
	p.state.ApplyResult(job.HostKey, res, err)
	if p.notify != nil {
		p.notify()
	}
}

func (p *WorkerPool) Submit(job PingJob) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.jobs <- job:
		return true
	}
}

func (p *WorkerPool) Close() {
	p.cancel()
	p.wg.Wait()
}

// SchedulerGroup owns per-host scheduler goroutines that enqueue jobs.
type SchedulerGroup struct {
	ctx     context.Context
	cancel  context.CancelFunc
	state   *state.SharedState
	pool    *WorkerPool
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewSchedulerGroup(ctx context.Context, st *state.SharedState, pool *WorkerPool) *SchedulerGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &SchedulerGroup{
		ctx:     ctx,
		cancel:  cancel,
		state:   st,
		pool:    pool,
		cancels: make(map[string]context.CancelFunc),
	}
}

func (g *SchedulerGroup) Start(hostKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.cancels[hostKey]; exists {
		return
	}
	ctx, cancel := context.WithCancel(g.ctx)
	g.cancels[hostKey] = cancel
	go g.runScheduler(ctx, hostKey)
}

func (g *SchedulerGroup) runScheduler(ctx context.Context, hostKey string) {
	// initial immediate ping
	g.enqueue(hostKey)
	for {
		name, interval, timeout, ok := g.state.HostConfig(hostKey)
		if !ok {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			g.pool.Submit(PingJob{HostKey: hostKey, HostName: name, Timeout: timeout})
		}
	}
}

func (g *SchedulerGroup) enqueue(hostKey string) {
	name, _, timeout, ok := g.state.HostConfig(hostKey)
	if !ok {
		return
	}
	g.pool.Submit(PingJob{HostKey: hostKey, HostName: name, Timeout: timeout})
}

func (g *SchedulerGroup) Stop(hostKey string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if cancel, ok := g.cancels[hostKey]; ok {
		cancel()
		delete(g.cancels, hostKey)
	}
}

func (g *SchedulerGroup) StopAll() {
	g.cancel()
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, c := range g.cancels {
		c()
		delete(g.cancels, k)
	}
}
