package tools

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"icarosu/internal/llm"
)

// RegisterBuiltins 注册一组开箱即用的小工具，用于让 LLM 输出更丰富。
func RegisterBuiltins(r *Registry) {
	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_current_time",
				Description: "获取当前的日期和具体时间",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return time.Now().Format("2006-01-02 15:04:05 Monday"), nil
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_today_date",
				Description: "获取今天的日期",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return time.Now().Format("2006-01-02"), nil
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "roll_dice",
				Description: "掷一个骰子，返回 1 到 sides 之间的随机数",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"sides": map[string]any{
							"type":        "integer",
							"description": "骰子面数，默认 6",
						},
					},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			sides := 6
			if v, ok := args["sides"]; ok {
				switch n := v.(type) {
				case float64:
					sides = int(n)
				case int:
					sides = n
				case string:
					parsed, err := strconv.Atoi(n)
					if err != nil {
						return "", fmt.Errorf("invalid sides: %v", n)
					}
					sides = parsed
				}
			}
			if sides <= 0 {
				return "", fmt.Errorf("sides must be positive")
			}
			return fmt.Sprintf("%d", rand.Intn(sides)+1), nil
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_random_emoji",
				Description: "获取一个随机的 QQ 风格表情/颜文字",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			emojis := []string{"😂", "🤣", "😏", "😴", "🐟", "🍉", "🎮", "📺", "(｡•̀ᴗ-)✧", "_(:з」∠)_", "歪歪.jpg", "吃瓜.jpg"}
			return emojis[rand.Intn(len(emojis))], nil
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_meme_text",
				Description: "获取一个适合当前群聊的梗/段子短句",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			memes := []string{
				"摸鱼一时爽，一直摸鱼一直爽",
				"别卷了别卷了，再卷我要报警了",
				"啊这……你们继续，我吃瓜",
				"笑死，这也能吵起来",
				"躺平是不可能躺平的，这辈子都不可能躺平的，除非周日",
			}
			return memes[rand.Intn(len(memes))], nil
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "send_image",
				Description: "发送一张图片到当前群聊/私聊，file 可以是本地路径或 URL",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"file": map[string]any{"type": "string", "description": "图片路径或URL"},
					},
					"required": []string{"file"},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "", fmt.Errorf("send_image requires chat context")
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "send_meme",
				Description: "从机器人的表情包里随机选一张表情包发送到当前群聊/私聊",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "", fmt.Errorf("send_meme requires chat context")
		},
	})

	r.Register(Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "get_random_number",
				Description: "获取一个指定范围内的随机整数",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"min": map[string]any{"type": "integer", "description": "最小值，默认 1"},
						"max": map[string]any{"type": "integer", "description": "最大值，默认 100"},
					},
					"required": []string{"max"},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			minVal := 1
			maxVal := 100
			if v, ok := args["min"]; ok {
				switch n := v.(type) {
				case float64:
					minVal = int(n)
				case int:
					minVal = n
				}
			}
			if v, ok := args["max"]; ok {
				switch n := v.(type) {
				case float64:
					maxVal = int(n)
				case int:
					maxVal = n
				}
			}
			if maxVal < minVal {
				return "", fmt.Errorf("max must be >= min")
			}
			return fmt.Sprintf("%d", rand.Intn(maxVal-minVal+1)+minVal), nil
		},
	})
}
