package kernel

import (
	"context"
	"errors"
	"sync"
	"time"

	"icarosu/internal/sdk"
)

var (
	ErrSchedulerClosed = errors.New("scheduler is closed")
	ErrBackpressure    = errors.New("scheduler queue is full")
)

// Scheduler 为每个会话保序、跨会话并行，并用全局 worker 信号量限制并发。
type Scheduler struct {
	ctx         context.Context
	cancel      context.CancelFunc
	handler     func(context.Context, sdk.Event) error
	workers     int
	buffer      int
	sem         chan struct{}
	idleTTL     time.Duration
	janitorC    context.CancelFunc
	janitorDone chan struct{}

	mu     sync.Mutex
	queues map[sdk.ConversationID]*conversationQueue
	closed bool
	wg     sync.WaitGroup
}

type conversationQueue struct {
	mu         sync.Mutex
	cond       *sync.Cond
	jobs       chan sdk.Event
	done       chan struct{}
	closed     bool
	active     int
	processing bool
	lastUsed   time.Time
}

func newConversationQueue(buffer int) *conversationQueue {
	q := &conversationQueue{
		jobs:     make(chan sdk.Event, buffer),
		done:     make(chan struct{}),
		lastUsed: time.Now(),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func NewScheduler(ctx context.Context, workers, buffer int, handler func(context.Context, sdk.Event) error) *Scheduler {
	return NewSchedulerWithIdle(ctx, workers, buffer, 10*time.Minute, handler)
}

// NewSchedulerWithIdle 允许自定义空闲会话队列回收时间（测试/运维用）。
func NewSchedulerWithIdle(ctx context.Context, workers, buffer int, idleTTL time.Duration, handler func(context.Context, sdk.Event) error) *Scheduler {
	if workers <= 0 {
		workers = 4
	}
	if buffer <= 0 {
		buffer = 32
	}
	if idleTTL <= 0 {
		idleTTL = 10 * time.Minute
	}
	sctx, cancel := context.WithCancel(ctx)
	s := &Scheduler{
		ctx:         sctx,
		cancel:      cancel,
		handler:     handler,
		workers:     workers,
		buffer:      buffer,
		idleTTL:     idleTTL,
		janitorDone: make(chan struct{}),
		sem:         make(chan struct{}, workers),
		queues:      make(map[sdk.ConversationID]*conversationQueue),
	}
	jctx, jcancel := context.WithCancel(context.Background())
	s.janitorC = jcancel
	go s.janitor(jctx)
	return s
}

func (s *Scheduler) janitor(ctx context.Context) {
	defer close(s.janitorDone)
	interval := s.idleTTL / 2
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.mu.Lock()
			for id, q := range s.queues {
				q.mu.Lock()
				if !q.closed && !q.processing && q.active == 0 && len(q.jobs) == 0 && now.Sub(q.lastUsed) > s.idleTTL {
					q.closed = true
					close(q.done)
					delete(s.queues, id)
				}
				q.mu.Unlock()
			}
			s.mu.Unlock()
		}
	}
}

func (s *Scheduler) QueueCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queues)
}

func (s *Scheduler) Submit(ctx context.Context, ev sdk.Event) error {
	for attempt := 0; attempt < 2; attempt++ {
		q, err := s.queueFor(ev.Conversation.ID)
		if err != nil {
			return err
		}
		err = q.submit(ctx, ev)
		if !errors.Is(err, ErrSchedulerClosed) {
			return err
		}
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			return ErrSchedulerClosed
		}
		// 队列被 janitor 回收；换新队列重试。
	}
	return ErrSchedulerClosed
}

func (s *Scheduler) queueFor(id sdk.ConversationID) (*conversationQueue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrSchedulerClosed
	}
	if q, ok := s.queues[id]; ok {
		return q, nil
	}
	q := newConversationQueue(s.buffer)
	s.queues[id] = q
	s.wg.Add(1)
	go s.run(q)
	return q, nil
}

func (s *Scheduler) run(q *conversationQueue) {
	defer s.wg.Done()
	for {
		select {
		case ev := <-q.jobs:
			s.process(q, ev)
		case <-q.done:
			// 关闭后不再接收新提交；等所有正在提交的 goroutine 退出，
			// 再排空已经进入队列的事件，避免丢事件。
			q.mu.Lock()
			for q.active > 0 {
				q.cond.Wait()
			}
			remaining := len(q.jobs)
			q.mu.Unlock()
			for i := 0; i < remaining; i++ {
				s.process(q, <-q.jobs)
			}
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Scheduler) process(q *conversationQueue, ev sdk.Event) {
	select {
	case s.sem <- struct{}{}:
	case <-s.ctx.Done():
		return
	}
	defer func() { <-s.sem }()

	q.mu.Lock()
	q.processing = true
	q.lastUsed = time.Now()
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.processing = false
		q.lastUsed = time.Now()
		q.mu.Unlock()
	}()

	if s.handler == nil {
		return
	}
	defer func() {
		_ = recover() // 单个 handler panic 不应杀死 worker；不在这里记录日志（无 logger）。
	}()
	_ = s.handler(s.ctx, ev)
}

func (q *conversationQueue) submit(ctx context.Context, ev sdk.Event) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrSchedulerClosed
	}
	q.active++
	q.lastUsed = time.Now()
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		q.active--
		if q.active == 0 {
			q.cond.Broadcast()
		}
		q.mu.Unlock()
	}()

	select {
	case q.jobs <- ev:
		return nil
	case <-q.done:
		return ErrSchedulerClosed
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 非阻塞背压：让 adapter 丢弃/记录过载事件，而不是为每条消息
		// 生成一个永久阻塞的 goroutine。
		return ErrBackpressure
	}
}

func (q *conversationQueue) close() {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.done)
	}
	for q.active > 0 {
		q.cond.Wait()
	}
	q.mu.Unlock()
}

// Close 停止接收新事件，排空已有队列后取消 worker。
func (s *Scheduler) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.janitorC != nil {
		s.janitorC()
		<-s.janitorDone
	}
	queues := make([]*conversationQueue, 0, len(s.queues))
	for _, q := range s.queues {
		queues = append(queues, q)
	}
	s.mu.Unlock()

	for _, q := range queues {
		q.close()
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		s.cancel()
		return ctx.Err()
	}
	s.cancel()
	return nil
}
