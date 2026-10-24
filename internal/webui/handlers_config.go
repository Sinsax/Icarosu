package webui

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"icarosu/internal/config"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSON(w, s.manager.Get())
	case http.MethodPost, http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
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

		if err := config.WriteSplit(&cfg, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			_ = os.WriteFile(path, old, 0o644)
			if llmPath != "" && oldLLM != nil {
				_ = os.WriteFile(llmPath, oldLLM, 0o644)
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated via webui form")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.manager.Get(),
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
		_, _ = w.Write(data)
	case http.MethodPost:
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

		if err := config.WriteSplit(&cfg, path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.manager.Reload(); err != nil {
			_ = os.WriteFile(path, old, 0o644)
			if llmPath != "" && oldLLM != nil {
				_ = os.WriteFile(llmPath, oldLLM, 0o644)
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.logger.Info("config updated and reloaded via webui")
		s.writeJSON(w, map[string]any{
			"ok":     true,
			"config": s.manager.Get(),
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
	if err := s.manager.Reload(); err != nil {
		s.logger.Warn("config reload failed", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.logger.Info("config reloaded via webui")
	s.writeJSON(w, map[string]any{
		"ok":     true,
		"config": s.manager.Get(),
	})
}
