package membersync_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"icarosu/internal/plugins/reflection/membersync"
	"icarosu/internal/sdk"
	"icarosu/internal/sdk/testkit"
)

func TestSyncAndProvide(t *testing.T) {
	host := testkit.NewFakeHost()
	host.MemberValue = &testkit.FakeMemberDirectory{Members: []sdk.Member{{ID: "qq:user:200", Name: "小明"}}}
	p := membersync.New().(*membersync.Plugin)
	if err := p.Init(context.Background(), host); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = p.Stop(context.Background()) }()

	p.Observe(context.Background(), sdk.Event{
		Kind:         "message.created",
		Conversation: sdk.Conversation{ID: "qq:group:1", Kind: "group", AdapterID: "onebot"},
	})
	deadline := time.Now().Add(time.Second)
	for {
		blocks, _ := p.Provide(context.Background(), &sdk.Turn{Event: sdk.Event{Conversation: sdk.Conversation{ID: "qq:group:1"}}})
		if len(blocks) > 0 && strings.Contains(blocks[0].Content, "小明") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("membersync did not produce context")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
