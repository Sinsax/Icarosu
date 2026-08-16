package meme

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Store 是本地表情包库。
type Store struct {
	dir   string
	mu    sync.RWMutex
	items []string
}

// Open 打开（或创建）表情包目录，并扫描已有文件。
func Open(dir string) (*Store, error) {
	if dir == "" {
		dir = "data/memes"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create meme dir: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read meme dir: %w", err)
	}

	s := &Store{dir: dir}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		s.items = append(s.items, filepath.Join(dir, entry.Name()))
	}
	return s, nil
}

// SaveFromURL 下载一张图片并保存到表情库。
func (s *Store) SaveFromURL(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty image url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download image returned %d", resp.StatusCode)
	}

	ext := extFromURL(url)
	name := fmt.Sprintf("meme_%d_%04d%s", time.Now().UnixNano(), rand.Intn(10000), ext)
	path := filepath.Join(s.dir, name)

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("save image: %w", err)
	}

	s.mu.Lock()
	s.items = append(s.items, path)
	s.mu.Unlock()
	return path, nil
}

// Random 随机返回一个表情包文件路径。
func (s *Store) Random() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		return "", fmt.Errorf("meme library is empty")
	}
	return s.items[rand.Intn(len(s.items))], nil
}

// Read 读取表情包文件内容。
func (s *Store) Read(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// Count 返回表情包数量。
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

func extFromURL(raw string) string {
	path := raw
	if idx := strings.Index(raw, "?"); idx >= 0 {
		path = raw[:idx]
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return ext
	default:
		return ".img"
	}
}
