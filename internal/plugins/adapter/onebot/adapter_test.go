package onebot_test

import (
	"context"
	"encoding/json"
	"testing"

	"icarosu/internal/plugins/adapter/onebot"
	"icarosu/internal/qq"
	"icarosu/internal/sdk"
)

type fakeClient struct {
	handler   qq.MessageHandler
	group     []any
	private   []any
	closed    bool
	connected bool
}

func (f *fakeClient) SetMessageHandler(h qq.MessageHandler) { f.handler = h }
func (f *fakeClient) Connect(ctx context.Context) error     { f.connected = true; return nil }
func (f *fakeClient) Close() error                          { f.closed = true; return nil }
func (f *fakeClient) SendGroupRaw(ctx context.Context, groupID int64, message any) error {
	f.group = append(f.group, message)
	return nil
}
func (f *fakeClient) SendPrivateRaw(ctx context.Context, userID int64, message any) error {
	f.private = append(f.private, message)
	return nil
}
func (f *fakeClient) GetGroupMemberList(ctx context.Context, groupID int64) ([]qq.GroupMember, error) {
	return []qq.GroupMember{{UserID: 200, Nickname: "小明", Card: "群名片", Role: "member"}}, nil
}

func groupMessage() qq.Event {
	message, _ := json.Marshal([]qq.Segment{
		{Type: "text", Data: map[string]any{"text": "早上好"}},
		{Type: "at", Data: map[string]any{"qq": float64(1)}},
		{Type: "image", Data: map[string]any{"url": "https://img.example/a.png"}},
		{Type: "reply", Data: map[string]any{"id": float64(456)}},
	})
	return qq.Event{
		PostType:    "message",
		MessageType: "group",
		SelfID:      1,
		GroupID:     100,
		UserID:      200,
		MessageID:   9,
		Message:     message,
		RawMessage:  "早上好",
		Sender:      qq.Sender{Nickname: "小明", Card: "群名片"},
	}
}

func TestNormalizeGroupMessage(t *testing.T) {
	ev, ok := onebot.Normalize(groupMessage())
	if !ok {
		t.Fatal("expected message to normalize")
	}
	if ev.Conversation.ID != "qq:group:100" || ev.Conversation.Kind != "group" {
		t.Fatalf("unexpected conversation: %+v", ev.Conversation)
	}
	if ev.Actor.ID != "qq:user:200" || ev.Actor.Name != "群名片" {
		t.Fatalf("unexpected actor: %+v", ev.Actor)
	}
	if ev.Self != "qq:user:1" {
		t.Fatalf("unexpected self: %q", ev.Self)
	}
	if len(ev.Parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(ev.Parts))
	}
	if ev.ReplyTo != "456" {
		t.Fatalf("expected reply to 456, got %q", ev.ReplyTo)
	}
	if len(ev.Mentions) != 1 || ev.Mentions[0] != "qq:user:1" {
		t.Fatalf("unexpected mentions: %+v", ev.Mentions)
	}
	if ev.Meta["native_message_id"] != "9" || ev.Meta["native_group_id"] != "100" {
		t.Fatalf("unexpected meta: %+v", ev.Meta)
	}
}

func TestNormalizeMarksAdmin(t *testing.T) {
	ev, ok := onebot.NormalizeWithAdmins(groupMessage(), map[int64]bool{200: true})
	if !ok {
		t.Fatal("expected admin message to normalize")
	}
	if !ev.Actor.IsAdmin {
		t.Fatal("expected actor to be marked admin")
	}
}

func TestNormalizeLargeAtID(t *testing.T) {
	message, _ := json.Marshal([]qq.Segment{{Type: "at", Data: map[string]any{"qq": float64(1234567890)}}})
	ev, ok := onebot.Normalize(qq.Event{
		PostType:    "message",
		MessageType: "group",
		SelfID:      1,
		GroupID:     100,
		UserID:      200,
		MessageID:   10,
		Message:     message,
	})
	if !ok {
		t.Fatal("expected normalize")
	}
	if len(ev.Mentions) != 1 || ev.Mentions[0] != "qq:user:1234567890" {
		t.Fatalf("unexpected mentions: %+v", ev.Mentions)
	}
	if ev.Parts[0].Meta["qq"] != "1234567890" {
		t.Fatalf("unexpected at part: %+v", ev.Parts[0])
	}
}

func TestNormalizeDropsSelfMessage(t *testing.T) {
	evt := groupMessage()
	evt.SelfID = 200
	evt.UserID = 200
	if _, ok := onebot.Normalize(evt); ok {
		t.Fatal("self message should be dropped")
	}
}

func TestNormalizeRecall(t *testing.T) {
	ev, ok := onebot.Normalize(qq.Event{
		PostType:   "notice",
		NoticeType: "group_recall",
		SelfID:     1,
		GroupID:    100,
		UserID:     200,
		MessageID:  9,
		OperatorID: 300,
	})
	if !ok {
		t.Fatal("expected notice to normalize")
	}
	if ev.Kind != "notice.recalled" || ev.Meta["native_operator_id"] != "300" {
		t.Fatalf("unexpected recall event: %+v", ev)
	}
}

func TestSendBuildsSegments(t *testing.T) {
	client := &fakeClient{}
	a := onebot.New(client, nil)
	action := sdk.Action{
		Conversation: sdk.Conversation{
			ID:        "qq:group:100",
			AdapterID: "onebot",
			Kind:      "group",
			Metadata:  map[string]string{"native_id": "100"},
		},
		ReplyTo: "456",
		Parts: []sdk.Part{
			{Type: sdk.PartText, Text: "[CQ:at,qq=all] 嗯"},
			{Type: sdk.PartImage, URL: "https://img.example/a.png"},
		},
	}
	if err := a.Send(context.Background(), action); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(client.group) != 1 {
		t.Fatalf("expected 1 send, got %d", len(client.group))
	}
	segments, ok := client.group[0].([]map[string]any)
	if !ok || len(segments) != 3 {
		t.Fatalf("unexpected segments: %#v", client.group[0])
	}
	if segments[0]["type"] != "reply" || segments[1]["type"] != "text" || segments[2]["type"] != "image" {
		t.Fatalf("unexpected segment order: %#v", segments)
	}
	text := segments[1]["data"].(map[string]any)["text"].(string)
	if text != "[CQ:at,qq=all] 嗯" {
		t.Fatalf("text should stay literal, got %q", text)
	}
}

func TestAdapterStartEmitsNormalizedEvent(t *testing.T) {
	client := &fakeClient{}
	a := onebot.New(client, nil)
	var got sdk.Event
	if err := a.Start(context.Background(), func(ctx context.Context, ev sdk.Event) {
		got = ev
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !client.connected || client.handler == nil {
		t.Fatal("adapter should connect and set handler")
	}
	client.handler(context.Background(), groupMessage())
	if got.Conversation.ID != "qq:group:100" {
		t.Fatalf("unexpected emitted event: %+v", got)
	}
}
