package tools_test

import (
	"context"
	"fmt"
	"testing"

	"icarosu/internal/llm"
	"icarosu/internal/tools"
)

func TestRegistryExecute(t *testing.T) {
	r := tools.New()
	r.Register(tools.Tool{
		Definition: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name: "add",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"a": map[string]any{"type": "integer"},
						"b": map[string]any{"type": "integer"},
					},
				},
			},
		},
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			a := int(args["a"].(float64))
			b := int(args["b"].(float64))
			return fmt.Sprintf("%d", a+b), nil
		},
	})

	defs := r.Definitions()
	if len(defs) != 1 || defs[0].Function.Name != "add" {
		t.Fatalf("unexpected definitions: %+v", defs)
	}

	got, err := r.Execute(context.Background(), "add", `{"a":1,"b":2}`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got != "3" {
		t.Fatalf("unexpected result: %q", got)
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	r := tools.New()
	if _, err := r.Execute(context.Background(), "missing", "{}"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
