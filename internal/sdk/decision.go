package sdk

import "time"

// DecisionKind 是回合决策类型。
type DecisionKind string

const (
	DecisionReply   DecisionKind = "reply"
	DecisionSilent  DecisionKind = "silent"
	DecisionObserve DecisionKind = "observe"
	DecisionDefer   DecisionKind = "defer"
)

// Decision 是 Decider 对一条事件的决策。
type Decision struct {
	Kind       DecisionKind
	Reason     string
	Confidence float64
	Defer      *Deferral
}

// Deferral 描述“稍后再看”的约束。
type Deferral struct {
	After       time.Duration
	Reason      string
	CancelKey   string
	MaxAttempts int
}

// TurnResult 是一个回合的结果。
type TurnResult struct {
	TurnID   TurnID
	Decision Decision
	Actions  []Action
	Trace    *Trace
}
