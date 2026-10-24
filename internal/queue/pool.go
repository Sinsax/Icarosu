package queue

import (
	"context"
	"sync"
)

// Job 是需要放入队列执行的任务。
type Job func(ctx context.Context)

// Pool 是一个有界队列 + 固定 worker 数的工作池。
// 用于限制并发处理数，避免事件/LLM 任务无限并发。
type Pool struct {
	workers   int
	jobs      chan Job
	wg        sync.WaitGroup
	closeOnce sync.Once
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
	}
	p.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go p.worker()
	}
	return p
}

// Submit 把一个任务放入队列；如果队列已满则阻塞，直到有 worker 取走或 ctx 取消。
func (p *Pool) Submit(ctx context.Context, job Job) error {
	if job == nil {
		return nil
	}
	select {
	case p.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close 停止接收新任务，并等待已提交任务全部完成。可安全调用多次。
func (p *Pool) Close() {
	p.closeOnce.Do(func() {
		close(p.jobs)
		p.wg.Wait()
	})
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		job(context.Background())
	}
}
