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
}

// NewBuffer 创建一个最多保存 max 行的日志缓冲。
func NewBuffer(max int) *Buffer {
	if max <= 0 {
		max = 500
	}
	return &Buffer{max: max}
}

// Write 实现 io.Writer，把日志文本写入环形缓冲。
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
	}
	return len(p), nil
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
