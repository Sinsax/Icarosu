package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"icarosu/internal/config"
)

type configView struct {
	config.Config
	Secrets map[string]bool `json:"_secrets"`
}

func (s *Server) configView() configView {
	cfg := s.manager.Get()
	redacted := config.Redact(cfg)
	return configView{
		Config: *redacted,
		Secrets: map[string]bool{
			"llm_api_key":         cfg.LLM.APIKey != "",
			"vlm_api_key":         cfg.VLM.APIKey != "",
			"onebot_access_token": cfg.QQ.OneBot.AccessToken != "",
			"webui_token":         cfg.WebUI.Token != "",
		},
	}
}

var secretYAMLLine = regexp.MustCompile(`(?m)^(\s*(?:api_key|access_token|token)\s*:\s*).*$`)

// redactRawConfig blanks secret-looking YAML values while preserving comments/structure.
// Secrets left blank on save are restored by config.PreserveSecrets.
func redactRawConfig(data []byte) []byte {
	redacted := secretYAMLLine.ReplaceAll(data, []byte(`$1""`))
	return append([]byte("# 敏感字段已脱敏；保存时留空会保留旧值。\n"), redacted...)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSON(w, s.configView())
	case http.MethodPost, http.MethodPut:
		s.configMu.Lock()
		defer s.configMu.Unlock()
		var cfg config.Config
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		config.PreserveSecrets(s.manager.Get(), &cfg)
		if cfg.Version == 0 {
			cfg.Version = config.CurrentVersion
		}
		if cfg.LLMConfigFile == "" {
			cfg.LLMConfigFile = s.manager.Get().LLMConfigFile
		}

		path := s.manager.Path()
		old, _ := os.ReadFile(path)
		var oldLLM []byte
		llmPath := ""
		if cfg.LLMConfigFile != "" {
			llmPath = config.ProviderPath(path, cfg.LLMConfigFile)
			oldLLM, _ = os.ReadFile(llmPath)
		}

		if err := cfg.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateProviderPath(path, cfg.LLMConfigFile); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.WriteSplit(&cfg, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			if old != nil {
				_ = os.WriteFile(path, old, 0o644)
			}
			if llmPath != "" && oldLLM != nil {
				_ = os.WriteFile(llmPath, oldLLM, 0o644)
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated via webui form")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.configView(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleConfigRaw(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(s.manager.Path())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(redactRawConfig(data))
	case http.MethodPost:
		s.configMu.Lock()
		defer s.configMu.Unlock()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(string(body)) == "" {
			http.Error(w, "config cannot be empty", http.StatusBadRequest)
			return
		}

		var cfg config.Config
		if err := yaml.Unmarshal(body, &cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		config.PreserveSecrets(s.manager.Get(), &cfg)
		if cfg.Version == 0 {
			cfg.Version = config.CurrentVersion
		}
		if cfg.LLMConfigFile == "" {
			cfg.LLMConfigFile = s.manager.Get().LLMConfigFile
		}

		path := s.manager.Path()
		old, _ := os.ReadFile(path)
		var oldLLM []byte
		llmPath := ""
		if cfg.LLMConfigFile != "" {
			llmPath = config.ProviderPath(path, cfg.LLMConfigFile)
			oldLLM, _ = os.ReadFile(llmPath)
		}

		if err := cfg.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidateProviderPath(path, cfg.LLMConfigFile); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		restored, err := config.RestoreSecretsYAML(body, s.manager.Get())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.WriteRawFile(path, restored); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			if old != nil {
				_ = config.WriteRawFile(path, old)
			}
			if llmPath != "" && oldLLM != nil {
				_ = os.WriteFile(llmPath, oldLLM, 0o644)
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated and reloaded via webui")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.configView(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.configMu.Lock()
	defer s.configMu.Unlock()
	if err := s.manager.Reload(); err != nil {
		s.logger.Warn("config reload failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.logger.Info("config reloaded via webui")
	s.writeJSON(w, map[string]any{
		"ok":     true,
		"config": s.configView(),
	})
}
