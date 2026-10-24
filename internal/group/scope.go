// Package group 提供群维度隔离与互通组解析。
package group

import "icarosu/internal/config"

// Resolver 负责把一个群映射到它实际可访问的数据范围。
type Resolver struct {
	memoryGroups     map[int64][]int64
	expressionGroups map[int64][]int64
}

// NewResolver 根据配置构造互通组解析器。
func NewResolver(memoryShareGroups, expressionShareGroups []config.ShareGroup) *Resolver {
	r := &Resolver{
		memoryGroups:     make(map[int64][]int64),
		expressionGroups: make(map[int64][]int64),
	}
	for _, g := range memoryShareGroups {
		r.addGroup(g.GroupIDs, r.memoryGroups)
	}
	for _, g := range expressionShareGroups {
		r.addGroup(g.GroupIDs, r.expressionGroups)
	}
	return r
}

func (r *Resolver) addGroup(groupIDs []int64, dst map[int64][]int64) {
	set := make(map[int64]struct{})
	for _, id := range groupIDs {
		if id == 0 {
			continue
		}
		set[id] = struct{}{}
	}
	for id := range set {
		ids := make([]int64, 0, len(set))
		for other := range set {
			ids = append(ids, other)
		}
		dst[id] = ids
	}
}

// MemoryGroupIDs 返回 groupID 在记忆互通组内可访问的所有群号（包含自己）。
func (r *Resolver) MemoryGroupIDs(groupID int64) []int64 {
	return r.groupIDs(groupID, r.memoryGroups)
}

// ExpressionGroupIDs 返回 groupID 在表达方式互通组内可访问的所有群号（包含自己）。
func (r *Resolver) ExpressionGroupIDs(groupID int64) []int64 {
	return r.groupIDs(groupID, r.expressionGroups)
}

func (r *Resolver) groupIDs(groupID int64, m map[int64][]int64) []int64 {
	if ids, ok := m[groupID]; ok {
		return ids
	}
	return []int64{groupID}
}