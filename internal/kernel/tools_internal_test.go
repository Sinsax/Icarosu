package kernel

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"

	"icarosu/internal/sdk"
)

func TestToolResultTruncationKeepsUTF8(t *testing.T) {
	r := newToolRegistry(slog.Default(), nil)
	if err := r.Register(sdk.Tool{
		Name: "big",
		Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
			return sdk.Text(strings.Repeat("汉", 6000)), nil
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	res, err := r.Execute(context.Background(), sdk.ToolCall{Name: "big"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !utf8.ValidString(res.Text) {
		t.Fatal("truncated tool result is not valid UTF-8")
	}
	if !strings.HasSuffix(res.Text, "[truncated]") {
		t.Fatalf("missing truncation marker: %q", res.Text[len(res.Text)-20:])
	}
}

func TestAdminToolPermission(t *testing.T) {
	r := newToolRegistry(slog.Default(), nil)
	if err := r.Register(sdk.Tool{
		Name:        "admin_only",
		Permissions: []string{"admin"},
		Execute: func(ctx context.Context, call sdk.ToolCall) (sdk.ToolResult, error) {
			return sdk.Text("ok"), nil
		},
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := r.Execute(context.Background(), sdk.ToolCall{Name: "admin_only"}); err == nil {
		t.Fatal("non-admin should be denied")
	}
	if _, err := r.Execute(context.Background(), sdk.ToolCall{Name: "admin_only", Actor: sdk.Actor{IsAdmin: true}}); err != nil {
		t.Fatalf("admin should be allowed: %v", err)
	}
}
