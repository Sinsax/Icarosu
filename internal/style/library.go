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

// Library 是表达方式库，支持全局默认和按群隔离。
type Library struct {
	mu          sync.RWMutex
	styles      []Style
	groupStyles map[int64][]Style
}

// NewLibrary 创建表达方式库。
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
		groupStyles: make(map[int64][]Style),
	}
}

// Add 添加一个全局表达方式。
func (l *Library) Add(tags []string, expr string) {
	if l == nil || strings.TrimSpace(expr) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.addLocked(&l.styles, tags, expr)
}

// AddForGroup 添加一个只属于某个群的表达方式。
func (l *Library) AddForGroup(groupID int64, tags []string, expr string) {
	if l == nil || strings.TrimSpace(expr) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	styles := l.groupStyles[groupID]
	l.addLocked(&styles, tags, expr)
	l.groupStyles[groupID] = styles
}

// AddForGroups 把表达方式同时添加到多个群（用于互通组）。
func (l *Library) AddForGroups(groupIDs []int64, tags []string, expr string) {
	for _, gid := range groupIDs {
		l.AddForGroup(gid, tags, expr)
	}
}

func (l *Library) addLocked(styles *[]Style, tags []string, expr string) {
	expr = strings.TrimSpace(expr)
	for i := range *styles {
		if hasAnyTag((*styles)[i].Tags, tags) {
			(*styles)[i].Templates = append((*styles)[i].Templates, expr)
			return
		}
	}
	*styles = append(*styles, Style{
		Tags:      append([]string(nil), tags...),
		Templates: []string{expr},
	})
}

// RandomByTags 从全局库随机选择一个表达方式。
func (l *Library) RandomByTags(tags []string) string {
	if l == nil {
		return ""
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return randomFrom(l.styles, tags)
}

// RandomByTagsForGroup 从某个群可见的表达方式中随机选择一个。
func (l *Library) RandomByTagsForGroup(groupID int64, tags []string) string {
	if l == nil {
		return ""
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return randomFrom(l.stylesForGroupLocked(groupID), tags)
}

// SampleByTags 从全局库随机选择最多 n 个表达方式。
func (l *Library) SampleByTags(tags []string, n int) []string {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return sampleFrom(l.styles, tags, n)
}

// SampleByTagsForGroup 从某个群可见的表达方式中随机选择最多 n 个。
func (l *Library) SampleByTagsForGroup(groupID int64, tags []string, n int) []string {
	if l == nil || n <= 0 {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return sampleFrom(l.stylesForGroupLocked(groupID), tags, n)
}

// Apply 把全局随机表达方式附加到回复末尾。
func (l *Library) Apply(reply string, tags []string) string {
	if l == nil {
		return reply
	}
	return applyExpr(reply, l.RandomByTags(tags))
}

// ApplyForGroup 把某个群的随机表达方式附加到回复末尾。
func (l *Library) ApplyForGroup(groupID int64, reply string, tags []string) string {
	if l == nil {
		return reply
	}
	return applyExpr(reply, l.RandomByTagsForGroup(groupID, tags))
}

func (l *Library) stylesForGroupLocked(groupID int64) []Style {
	if styles, ok := l.groupStyles[groupID]; ok {
		return append(append([]Style(nil), l.styles...), styles...)
	}
	return l.styles
}

func randomFrom(styles []Style, tags []string) string {
	var candidates []string
	for _, s := range styles {
		if hasAnyTag(s.Tags, tags) {
			candidates = append(candidates, s.Templates...)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rand.Intn(len(candidates))]
}

func sampleFrom(styles []Style, tags []string, n int) []string {
	var candidates []string
	for _, s := range styles {
		if hasAnyTag(s.Tags, tags) {
			candidates = append(candidates, s.Templates...)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	return candidates
}

func applyExpr(reply, expr string) string {
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
