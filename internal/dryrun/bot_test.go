package dryrun_test

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"icarosu/internal/dryrun"
	"icarosu/internal/plugin"
)

type countingBot struct {
	groupMsg   atomic.Int64
	privateMsg atomic.Int64
	groupImg   atomic.Int64
	privateImg atomic.Int64
}

func (b *countingBot) SendGroupMessage(_ context.Context, _ int64, _ string) error {
	b.groupMsg.Add(1)
	return nil
}
func (b *countingBot) SendPrivateMessage(_ context.Context, _ int64, _ string) error {
	b.privateMsg.Add(1)
	return nil
}
func (b *countingBot) SendGroupImage(_ context.Context, _ int64, _ string) error {
	b.groupImg.Add(1)
	return nil
}
func (b *countingBot) SendPrivateImage(_ context.Context, _ int64, _ string) error {
	b.privateImg.Add(1)
	return nil
}

func TestDryRunBotDoesNotSend(t *testing.T) {
	inner := &countingBot{}
	b := dryrun.New(inner, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	if err := b.SendGroupMessage(ctx, 1, "hello"); err != nil {
		t.Fatalf("SendGroupMessage: %v", err)
	}
	if err := b.SendPrivateMessage(ctx, 2, "hi"); err != nil {
		t.Fatalf("SendPrivateMessage: %v", err)
	}
	if err := b.SendGroupImage(ctx, 1, "file"); err != nil {
		t.Fatalf("SendGroupImage: %v", err)
	}
	if err := b.SendPrivateImage(ctx, 2, "file"); err != nil {
		t.Fatalf("SendPrivateImage: %v", err)
	}

	if inner.groupMsg.Load() != 0 || inner.privateMsg.Load() != 0 || inner.groupImg.Load() != 0 || inner.privateImg.Load() != 0 {
		t.Fatalf("dry run bot should not call inner sends: %+v", inner)
	}
}

var _ plugin.Bot = (*countingBot)(nil)
