package async

import (
	"context"
	"sync"
)

// Group 是带关闭语义的异步任务组：Close 后不再接受新任务，
// 避免 WaitGroup 的 Add 与 Wait 并发使用。
type Group struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// Go 启动一个任务；若已关闭则返回 false。
func (g *Group) Go(fn func()) bool {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return false
	}
	g.wg.Add(1)
	g.mu.Unlock()

	go func() {
		defer g.wg.Done()
		defer func() { _ = recover() }()
		fn()
	}()
	return true
}

// Close 停止接受新任务，并在 ctx 截止前等待已有任务。
func (g *Group) Close(ctx context.Context) error {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
