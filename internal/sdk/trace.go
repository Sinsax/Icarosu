package sdk

import "time"

// Trace 是一回合的可回放记录。
type Trace struct {
	TurnID         TurnID
	EventID        EventID
	ConversationID ConversationID
	ActorID        ActorID
	Decision       Decision
	Blocks         []TraceBlock
	LLM            []TraceLLM
	Tools          []TraceTool
	Actions        []TraceAction
	LatencyMS      int64
	CostUSD        float64
	Errors         []TraceError
	StartedAt      time.Time
}

type TraceBlock struct {
	Source   string
	Tokens   int
	Priority int
	Omitted  bool
}

type TraceLLM struct {
	Round     int
	Tools     []string
	LatencyMS int64
	Tokens    int
	Error     string
}

type TraceTool struct {
	Name      string
	LatencyMS int64
	OK        bool
	Error     string
}

type TraceAction struct {
	Adapter string
	Parts   int
	OK      bool
	Error   string
}

type TraceError struct {
	Stage   string
	Code    string
	Message string
}
