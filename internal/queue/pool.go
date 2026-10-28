package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// ErrClosed 表示 Pool 已关闭，无法再提交任务。
var ErrClosed = errors.New("queue: pool is closed")

// ErrFull 表示队列已满且 TrySubmit 不等待。
var ErrFull = errors.New("queue: pool is full")

// Job 是需要放入队列执行的任务。
type Job func(ctx context.Context)

// Pool 是一个有界队列 + 固定 worker 数的工作池。
// 用于限制并发处理数，避免事件/LLM 任务无限并发。
type Pool struct {
	workers int
	jobs    chan Job
	wg      sync.WaitGroup

	mu     sync.Mutex
	cond   *sync.Cond
	done   chan struct{}
	closed bool
	active int
}

// NewPool 创建一个工作池并立即启动 workers。
func NewPool(workers, buffer int) *Pool {
	if workers <= 0 {
		workers = 4
	}
	if buffer <= 0 {
		buffer = workers * 32
	}
	p := &Pool{
		workers: workers,
		jobs:    make(chan Job, buffer),
		done:    make(chan struct{}),
	}
	p.cond = sync.NewCond(&p.mu)
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}
	return p
}

// Submit 把一个任务放入队列；如果队列已满则阻塞，直到有 worker 取走、ctx 取消
// 或 Pool 关闭。任务执行时会拿到提交时的 ctx（而不是 Background）。
func (p *Pool) Submit(ctx context.Context, job Job) error {
	return p.enqueue(ctx, job, true)
}

// TrySubmit 与 Submit 相同，但队列满时立即返回 ErrFull，避免调用方阻塞。
func (p *Pool) TrySubmit(ctx context.Context, job Job) error {
	return p.enqueue(ctx, job, false)
}

func (p *Pool) enqueue(ctx context.Context, job Job, block bool) error {
	if job == nil {
		return nil
	}

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrClosed
	}
	p.active++
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.active--
		if p.active == 0 {
			p.cond.Broadcast()
		}
		p.mu.Unlock()
	}()

	wrapped := func(context.Context) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		job(ctx)
	}

	if block {
		select {
		case p.jobs <- wrapped:
			return nil
		case <-p.done:
			return ErrClosed
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case p.jobs <- wrapped:
		return nil
	case <-p.done:
		return ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
		return ErrFull
	}
}

// Close 停止接收新任务，并等待已提交任务全部完成。可安全调用多次。
func (p *Pool) Close() { _ = p.CloseWithContext(context.Background()) }

// CloseWithContext 与 Close 相同，但等待 worker 排空受 ctx 约束。
func (p *Pool) CloseWithContext(ctx context.Context) error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.done)
		for p.active > 0 {
			p.cond.Wait()
		}
		close(p.jobs)
	}
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("queue job panic", "panic", r)
				}
			}()
			job(context.Background())
		}()
	}
}
