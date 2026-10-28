package queue_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"icarosu/internal/queue"
)

type ctxKey string

func TestPoolExecutesAllJobs(t *testing.T) {
	p := queue.NewPool(2, 8)
	var count atomic.Int64
	var mu sync.Mutex
	done := make(chan struct{})

	for i := 0; i < 20; i++ {
		if err := p.Submit(context.Background(), func(ctx context.Context) {
			count.Add(1)
			mu.Lock()
			defer mu.Unlock()
			if count.Load() == 20 {
				close(done)
			}
		}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("not all jobs executed, count=%d", count.Load())
	}
	p.Close()
}

func TestPoolCloseUnblocksBlockedSubmit(t *testing.T) {
	p := queue.NewPool(1, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("submit block: %v", err)
	}
	<-started
	if err := p.Submit(context.Background(), func(context.Context) {}); err != nil {
		t.Fatalf("submit queued: %v", err)
	}

	blocked := make(chan error, 1)
	go func() { blocked <- p.Submit(context.Background(), func(context.Context) {}) }()
	time.Sleep(20 * time.Millisecond)

	closed := make(chan struct{})
	go func() {
		p.Close()
		close(closed)
	}()

	select {
	case err := <-blocked:
		if !errors.Is(err, queue.ErrClosed) {
			t.Fatalf("blocked Submit should return ErrClosed, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("blocked Submit did not unblock on Close")
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return")
	}
}

func TestPoolTrySubmitFull(t *testing.T) {
	p := queue.NewPool(1, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("submit blocker: %v", err)
	}
	<-started
	if err := p.Submit(context.Background(), func(context.Context) {}); err != nil {
		t.Fatalf("submit queued: %v", err)
	}
	if err := p.TrySubmit(context.Background(), func(context.Context) {}); !errors.Is(err, queue.ErrFull) {
		t.Fatalf("expected ErrFull, got %v", err)
	}
	close(release)
	p.Close()
}

func TestPoolCloseWithContextTimesOut(t *testing.T) {
	p := queue.NewPool(1, 1)
	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := p.CloseWithContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	close(release)
	p.Close()
}

func TestPoolCloseWaitsForRunningJobs(t *testing.T) {
	p := queue.NewPool(1, 1)
	started := make(chan struct{})
	release := make(chan struct{})

	if err := p.Submit(context.Background(), func(ctx context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	<-started
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()

	p.Close()
}

func TestPoolRejectsSubmitAfterClose(t *testing.T) {
	p := queue.NewPool(1, 1)
	p.Close()
	if err := p.Submit(context.Background(), func(context.Context) {}); !errors.Is(err, queue.ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestPoolConcurrentSubmitAndClose(t *testing.T) {
	p := queue.NewPool(2, 4)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = p.Submit(context.Background(), func(context.Context) {})
		}()
	}
	time.Sleep(10 * time.Millisecond)
	p.Close()
	wg.Wait()
}

func TestPoolJobGetsSubmitContext(t *testing.T) {
	p := queue.NewPool(1, 1)
	defer p.Close()

	got := make(chan string, 1)
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")
	if err := p.Submit(ctx, func(jobCtx context.Context) {
		got <- jobCtx.Value(ctxKey("k")).(string)
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	select {
	case v := <-got:
		if v != "v" {
			t.Fatalf("unexpected context value %q", v)
		}
	case <-time.After(time.Second):
		t.Fatal("job did not run")
	}
}

func TestPoolSkipsJobWithCanceledContext(t *testing.T) {
	p := queue.NewPool(1, 1)
	defer p.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	if err := p.Submit(context.Background(), func(context.Context) {
		close(started)
		<-release
	}); err != nil {
		t.Fatalf("submit blocker: %v", err)
	}
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	ran := make(chan struct{}, 1)
	if err := p.Submit(ctx, func(context.Context) { ran <- struct{}{} }); err != nil {
		t.Fatalf("submit canceled job: %v", err)
	}
	cancel()
	close(release)

	select {
	case <-ran:
		t.Fatal("canceled job should not run")
	case <-time.After(100 * time.Millisecond):
	}
}
