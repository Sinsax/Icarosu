package chat

import (
	"strings"
	"testing"
)

func TestPostprocessReplyRemovesListMarkers(t *testing.T) {
	input := "- 第一点\n- 第二点\n\n\n* 第三点"
	got := PostprocessReply(input)
	want := "第一点\n第二点\n\n第三点"
	if got != want {
		t.Fatalf("unexpected postprocess result:\n got: %q\nwant: %q", got, want)
	}
}

func TestPostprocessReplyTrimsSpace(t *testing.T) {
	got := PostprocessReply("  你好呀～  ")
	if got != "你好呀～" {
		t.Fatalf("unexpected trim result: %q", got)
	}
}

func TestPostprocessReplyRemovesAIPhrases(t *testing.T) {
	got := PostprocessReply("作为一个AI，我建议你早点休息，总的来说晚安好梦")
	if got == "" {
		t.Fatal("expected non-empty reply")
	}
	if strings.Contains(got, "作为一个AI") || strings.Contains(got, "总的来说") {
		t.Fatalf("AI phrases should be removed, got %q", got)
	}
}

func TestSmartTruncate(t *testing.T) {
	got, needPolish := SmartTruncate("一二三四五六七八九", 7)
	if got != "一二三四五六七" || !needPolish {
		t.Fatalf("unexpected hard cut result: %q needPolish=%v", got, needPolish)
	}

	got, needPolish = SmartTruncate("今天天气真好，我们出去玩", 7)
	if got != "今天天气真好" || needPolish {
		t.Fatalf("unexpected comma cut result: %q needPolish=%v", got, needPolish)
	}
}

func TestNaturalParts(t *testing.T) {
	parts := NaturalParts("在等外卖呢 你吃了没")
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %v", parts)
	}
	if parts[0] != "在等外卖呢" || parts[1] != "你吃了没" {
		t.Fatalf("unexpected split: %v", parts)
	}
}

func TestCleanMessageRemovesCQAt(t *testing.T) {
	got := CleanMessage("[CQ:at,qq=123] 你好")
	if got != "你好" {
		t.Fatalf("unexpected clean result: %q", got)
	}
}
