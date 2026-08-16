package queue_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"icarosu/internal/queue"
)

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
