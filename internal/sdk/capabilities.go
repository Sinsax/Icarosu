package sdk

import "context"

// Member 是群成员快照。
type Member struct {
	ID   ActorID
	Name string
	Card string
	Role string
}

// MemberDirectory 是可选的平台成员目录能力。
type MemberDirectory interface {
	ListMembers(ctx context.Context, conversation Conversation) ([]Member, error)
}

// MemberHost 是可选 Host 能力：读取当前 adapter 的成员目录。
type MemberHost interface {
	Members() MemberDirectory
}

// ImageAnalysis 是 VLM 图片理解结果。
type ImageAnalysis struct {
	Description string
	IsMeme      bool
	FitsPersona bool
	Tags        []string
	Name        string
	Reason      string
}

// Vision 是可选的图片理解能力。
type Vision interface {
	AnalyzeImage(ctx context.Context, url, persona string) (ImageAnalysis, error)
}

// VisionHost 是可选 Host 能力。
type VisionHost interface {
	Vision() Vision
}

// MemeStore 是可选的表情包收藏能力。
type MemeStore interface {
	SaveFromURL(ctx context.Context, url string) (string, error)
}

// MemeHost 是可选 Host 能力。
type MemeHost interface {
	Memes() MemeStore
}
