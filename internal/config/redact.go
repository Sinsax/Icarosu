package config

// Redact returns a shallow copy of cfg with secret fields blanked.
// The original config is not modified, so callers can safely use it for
// logging or WebUI responses.
func Redact(cfg *Config) *Config {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.LLM.APIKey = ""
	out.VLM.APIKey = ""
	out.QQ.OneBot.AccessToken = ""
	out.WebUI.Token = ""
	return &out
}

// PreserveSecrets keeps the old secret values when next leaves them empty.
// This lets WebUI forms and raw YAML editors submit a redacted config without
// wiping credentials that were intentionally hidden from the response.
func PreserveSecrets(old, next *Config) {
	if old == nil || next == nil {
		return
	}
	if next.LLM.APIKey == "" {
		next.LLM.APIKey = old.LLM.APIKey
	}
	if next.VLM.APIKey == "" {
		next.VLM.APIKey = old.VLM.APIKey
	}
	if next.QQ.OneBot.AccessToken == "" {
		next.QQ.OneBot.AccessToken = old.QQ.OneBot.AccessToken
	}
	if next.WebUI.Token == "" {
		next.WebUI.Token = old.WebUI.Token
	}
}
