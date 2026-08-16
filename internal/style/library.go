package style

import (
	"math/rand"
	"strings"
	"sync"
)

// Style 是一组带标签的表达方式。
type Style struct {
	Tags      []string
	Templates []string
}

// Library 是表达方式库，支持手动/自动学习填充。
type Library struct {
	mu     sync.RWMutex
	styles []Style
}

func NewLibrary() *Library {
	return &Library{
		styles: []Style{
			{
				Tags:      []string{"lazy", "night", "sleep"},
				Templates: []string{"晚安", "行吧", "嗯", "确实", "？"},
			},
			{
				Tags:      []string{"funny", "laugh", "meme"},
				Templates: []string{"哈哈", "笑死", "确实", "草"},
			},
			{
				Tags:      []string{"casual", "food", "daily"},
				Templates: []string{"没吃", "哈哈", "确实", "行吧"},
			},
			{
				Tags:      []string{"question", "curious"},
				Templates: []string{"？", "真的假的", "细说", "展开说说"},
			},
		},
	}
}

// Add 添加一个带标签的表达方式。
func (l *Library) Add(tags []string, expr string) {
	if l == nil || strings.TrimSpace(expr) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.styles {
		if hasAnyTag(l.styles[i].Tags, tags) {
			l.styles[i].Templates = append(l.styles[i].Templates, strings.TrimSpace(expr))
			return
		}
	}
	l.styles = append(l.styles, Style{
		Tags:      append([]string(nil), tags...),
		Templates: []string{strings.TrimSpace(expr)},
	})
}

// RandomByTags 根据标签随机选择一个表达方式。
func (l *Library) RandomByTags(tags []string) string {
	if l == nil {
		return ""
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	var candidates []string
	for _, s := range l.styles {
		if hasAnyTag(s.Tags, tags) {
			candidates = append(candidates, s.Templates...)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rand.Intn(len(candidates))]
}

// SampleByTags 根据标签随机选择最多 n 个表达方式。
func (l *Library) SampleByTags(tags []string, n int) []string {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	var candidates []string
	for _, s := range l.styles {
		if hasAnyTag(s.Tags, tags) {
			candidates = append(candidates, s.Templates...)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	// 随机打乱取前 n 个
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	return candidates
}

// Apply 把随机表达方式附加到回复末尾。
func (l *Library) Apply(reply string, tags []string) string {
	if l == nil {
		return reply
	}
	expr := l.RandomByTags(tags)
	if expr == "" {
		return reply
	}
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return expr
	}
	if strings.HasSuffix(reply, expr) {
		return reply
	}
	return reply + expr
}

func hasAnyTag(styleTags, tags []string) bool {
	for _, a := range styleTags {
		for _, b := range tags {
			if a == b {
				return true
			}
		}
	}
	return false
}
