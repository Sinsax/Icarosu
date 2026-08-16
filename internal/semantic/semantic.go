package semantic

import (
	"context"
	"hash/fnv"
	"log/slog"
	"math"
	"strings"

	"icarosu/internal/store"
)

// Embedder 把文本转换成向量。
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// LocalEmbedder 是一个不依赖外部 API 的轻量向量化器：
// 使用字符/二元组/英文词的哈希特征构造向量，适合本地语义检索。
type LocalEmbedder struct {
	dim int
}

func NewLocalEmbedder(dim int) *LocalEmbedder {
	if dim <= 0 {
		dim = 256
	}
	return &LocalEmbedder{dim: dim}
}

func (e *LocalEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, e.dim)
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return vec, nil
	}

	runes := []rune(text)
	add := func(parts ...string) {
		h := fnv.New32a()
		for _, p := range parts {
			_, _ = h.Write([]byte(p))
		}
		vec[h.Sum32()%uint32(e.dim)]++
	}

	for _, r := range runes {
		if !isSpace(r) {
			add(string(r))
		}
	}
	for i := 0; i+1 < len(runes); i++ {
		if !isSpace(runes[i]) && !isSpace(runes[i+1]) {
			add(string(runes[i]), string(runes[i+1]))
		}
	}

	// 英文/数字单词也作为特征，增强跨语言检索。
	for _, word := range strings.FieldsFunc(text, func(r rune) bool {
		return isSpace(r) || isPunct(r)
	}) {
		if word != "" {
			add("word", word)
		}
	}

	normalize(vec)
	return vec, nil
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isPunct(r rune) bool {
	return strings.ContainsRune("，。！？、；：,.!?;:'\"()[]{}<>《》【】", r)
}

func normalize(v []float32) {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}

// Service 是语义记忆服务：负责写入带向量的长期记忆，并按语义检索。
type Service struct {
	store    *store.Store
	embedder Embedder
	logger   *slog.Logger
}

func NewService(st *store.Store, embedder Embedder, logger *slog.Logger) *Service {
	if embedder == nil {
		embedder = NewLocalEmbedder(256)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: st, embedder: embedder, logger: logger}
}

// SaveMemory 保存一条长期记忆；向量化失败时退化为普通保存。
func (s *Service) SaveMemory(ctx context.Context, groupID, userID int64, content string) error {
	if s.store == nil {
		return nil
	}
	vec, err := s.embedder.Embed(ctx, content)
	if err == nil && len(vec) > 0 {
		_, err = s.store.SaveMemoryWithVector(groupID, userID, content, vec)
		if err == nil {
			return nil
		}
		s.logger.Warn("save memory with vector failed, fallback to plain", "error", err)
	}
	return s.store.SaveMemory(groupID, userID, content)
}

// Search 按语义相似度检索某个群的长期记忆。
func (s *Service) Search(ctx context.Context, groupID int64, query string, limit int) ([]store.MemoryRecord, error) {
	if s.store == nil {
		return nil, nil
	}
	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.store.SearchMemories(groupID, vec, limit)
}
