package vlm

import (
	"context"
	"fmt"

	"icarosu/internal/sdk"
)

// SDKVision 把 *Client 适配成 sdk.Vision。
type SDKVision struct {
	client *Client
}

func NewSDKVision(c *Client) *SDKVision { return &SDKVision{client: c} }

func (v *SDKVision) AnalyzeImage(ctx context.Context, url, persona string) (sdk.ImageAnalysis, error) {
	if v.client == nil {
		return sdk.ImageAnalysis{}, fmt.Errorf("vlm client is nil")
	}
	result, err := v.client.Analyze(ctx, url, persona)
	if err != nil {
		return sdk.ImageAnalysis{}, err
	}
	if result == nil {
		return sdk.ImageAnalysis{}, fmt.Errorf("vlm returned nil result")
	}
	return sdk.ImageAnalysis{
		Description: result.Description,
		IsMeme:      result.IsMeme,
		FitsPersona: result.FitsPersona,
		Tags:        append([]string(nil), result.Tags...),
		Name:        result.Name,
		Reason:      result.Reason,
	}, nil
}

var _ sdk.Vision = (*SDKVision)(nil)
