package kernel

import "sync/atomic"

// Metrics 是内核运行指标。
type Metrics struct {
	Turns        atomic.Int64
	TurnErrors   atomic.Int64
	ToolCalls    atomic.Int64
	ToolErrors   atomic.Int64
	Actions      atomic.Int64
	ActionErrors atomic.Int64
}

func (m *Metrics) Snapshot() map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	return map[string]int64{
		"turns_total":         m.Turns.Load(),
		"turn_errors_total":   m.TurnErrors.Load(),
		"tool_calls_total":    m.ToolCalls.Load(),
		"tool_errors_total":   m.ToolErrors.Load(),
		"actions_total":       m.Actions.Load(),
		"action_errors_total": m.ActionErrors.Load(),
	}
}
