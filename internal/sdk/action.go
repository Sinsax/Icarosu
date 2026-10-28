package sdk

// Action 是内核要发给 adapter 的动作。
type Action struct {
	ID             string
	TurnID         TurnID
	Conversation   Conversation
	Actor          ActorID // 发送者（通常是 Self）
	Parts          []Part
	ReplyTo        string
	IdempotencyKey string
	Silent         bool
	Meta           map[string]string
}
