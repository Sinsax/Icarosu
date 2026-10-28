package config_test

import (
	"testing"

	"icarosu/internal/config"
)

func TestRedactBlanksSecretsWithoutMutating(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.APIKey = "llm-key"
	cfg.VLM.APIKey = "vlm-key"
	cfg.QQ.OneBot.AccessToken = "onebot-token"
	cfg.WebUI.Token = "webui-token"
	cfg.Bot.Name = "小智"

	redacted := config.Redact(cfg)
	if redacted.LLM.APIKey != "" || redacted.VLM.APIKey != "" ||
		redacted.QQ.OneBot.AccessToken != "" || redacted.WebUI.Token != "" {
		t.Fatalf("secrets were not blanked: %+v", redacted)
	}
	if redacted.Bot.Name != "小智" {
		t.Fatalf("non-secret fields should be preserved")
	}
	if cfg.LLM.APIKey != "llm-key" || cfg.WebUI.Token != "webui-token" {
		t.Fatal("Redact must not mutate the original config")
	}
}

func TestPreserveSecretsFillsEmptyOnly(t *testing.T) {
	old := &config.Config{}
	old.LLM.APIKey = "old-llm"
	old.VLM.APIKey = "old-vlm"
	old.QQ.OneBot.AccessToken = "old-onebot"
	old.WebUI.Token = "old-webui"

	next := &config.Config{}
	next.LLM.APIKey = "new-llm"
	next.WebUI.Token = "new-webui"

	config.PreserveSecrets(old, next)
	if next.LLM.APIKey != "new-llm" {
		t.Fatalf("non-empty secret should not be overwritten, got %q", next.LLM.APIKey)
	}
	if next.VLM.APIKey != "old-vlm" || next.QQ.OneBot.AccessToken != "old-onebot" {
		t.Fatalf("empty secrets should be filled: %+v", next)
	}
	if next.WebUI.Token != "new-webui" {
		t.Fatalf("non-empty webui token should not be overwritten, got %q", next.WebUI.Token)
	}
}
