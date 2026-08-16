package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Buffer 是一个内存日志环形缓冲，可作为 io.Writer 使用。
type Buffer struct {
	mu     sync.Mutex
	lines  []string
	max    int
	subs   map[chan string]struct{}
}

// NewBuffer 创建一个最多保存 max 行的日志缓冲。
func NewBuffer(max int) *Buffer {
	if max <= 0 {
		max = 500
	}
	return &Buffer{max: max, subs: make(map[chan string]struct{})}
}

// Write 实现 io.Writer，把日志文本写入环形缓冲，并推送给订阅者。
func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, line := range strings.Split(string(p), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		b.lines = append(b.lines, line)
		if len(b.lines) > b.max {
			b.lines = b.lines[len(b.lines)-b.max:]
		}
		for ch := range b.subs {
			select {
			case ch <- line:
			default:
			}
		}
	}
	return len(p), nil
}

// Subscribe 订阅新的日志行。返回的取消函数会关闭订阅通道。
func (b *Buffer) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// Recent 返回最近最多 limit 行日志。
func (b *Buffer) Recent(limit int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if limit <= 0 || limit > len(b.lines) {
		limit = len(b.lines)
	}
	out := make([]string, limit)
	copy(out, b.lines[len(b.lines)-limit:])
	return out
}

// NewWithBuffer 创建同时输出到 stdout 和内存缓冲的 logger。
func NewWithBuffer(level string, size int) (*slog.Logger, *Buffer) {
	buf := NewBuffer(size)
	multi := io.MultiWriter(os.Stdout, buf)
	return NewWithWriter(level, multi), buf
}

// NewWithWriter 创建输出到指定 writer 的 logger。
func NewWithWriter(level string, w io.Writer) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: lvl,
	}))
}
