package webui

import (
	"encoding/json"
	"io"
	"net/http"

	"icarosu/internal/plugin"
)

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.plugins == nil {
		s.writeJSON(w, map[string]any{"plugins": []plugin.PluginState{}})
		return
	}
	s.writeJSON(w, map[string]any{"plugins": s.plugins.States()})
}

func (s *Server) handlePluginToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.plugins == nil {
		http.Error(w, "plugin manager not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.plugins.SetEnabled(req.Name, req.Enabled)
	s.logger.Info("plugin toggled", "name", req.Name, "enabled", req.Enabled)
	s.writeJSON(w, map[string]any{"ok": true})
}
