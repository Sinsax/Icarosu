package vlm

import "context"

// Analyzer 是上层业务依赖的 VLM 图片理解能力接口。
// 目前由 *Client 实现；测试或未来接入其他图片理解服务时可以提供替代实现。
type Analyzer interface {
	Analyze(ctx context.Context, imageURL, persona string) (*Result, error)
}

var _ Analyzer = (*Client)(nil)
