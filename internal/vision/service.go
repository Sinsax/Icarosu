package vision

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"icarosu/internal/config"
	"icarosu/internal/meme"
	"icarosu/internal/vlm"
)

// Service 是基于 VLM 的通用图片理解服务。
// 它既提供图片内容描述供群聊上下文使用，也提供“偷表情包”能力。
type Service struct {
	client vlm.Analyzer
	memes  *meme.Store
	cfg    config.MemeConfig

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	result  *vlm.Result
	expires time.Time
}

func New(client vlm.Analyzer, memes *meme.Store, cfg config.MemeConfig) *Service {
	return &Service{
		client: client,
		memes:  memes,
		cfg:    cfg,
		cache:  make(map[string]cacheEntry),
	}
}

// Analyze 识别图片，带短时缓存，避免同一张图被重复调用 VLM。
func (s *Service) Analyze(ctx context.Context, imageURL, persona string) (*vlm.Result, error) {
	if s.client == nil {
		return nil, fmt.Errorf("vlm client is nil")
	}

	s.mu.Lock()
	if entry, ok := s.cache[imageURL]; ok && time.Now().Before(entry.expires) {
		s.mu.Unlock()
		return entry.result, nil
	}
	s.mu.Unlock()

	result, err := s.client.Analyze(ctx, imageURL, persona)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.cache[imageURL] = cacheEntry{result: result, expires: time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	return result, nil
}

// Understand 返回适合放入群聊上下文的图片描述。
func (s *Service) Understand(ctx context.Context, imageURL, persona string) (string, error) {
	result, err := s.Analyze(ctx, imageURL, persona)
	if err != nil {
		return "", err
	}

	var parts []string
	if result.Description != "" {
		parts = append(parts, result.Description)
	}
	if result.IsMeme {
		parts = append(parts, "是表情包/梗图")
	}
	if result.Name != "" {
		parts = append(parts, "名称:"+result.Name)
	}
	if result.Reason != "" {
		parts = append(parts, result.Reason)
	}
	if len(result.Tags) > 0 {
		parts = append(parts, "标签:"+strings.Join(result.Tags, ","))
	}
	if len(parts) == 0 {
		parts = append(parts, "图片内容暂无法概括")
	}
	return strings.Join(parts, "；"), nil
}

// MaybeSteal 如果图片符合人设，按概率保存到表情库。
func (s *Service) MaybeSteal(ctx context.Context, imageURL, persona string) error {
	result, err := s.Analyze(ctx, imageURL, persona)
	if err != nil {
		return err
	}
	if !result.IsMeme || !result.FitsPersona {
		return nil
	}
	if s.cfg.StealProbability > 0 && rand.Float64() > s.cfg.StealProbability {
		return nil
	}
	if s.memes == nil {
		return nil
	}
	_, err = s.memes.SaveFromURL(ctx, imageURL)
	return err
}
