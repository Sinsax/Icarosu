package webui

import (
	"encoding/json"
	"io"
	"net/http"
)
func (s *Server) handleMemes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.memes == nil {
		s.writeJSON(w, map[string]any{"memes": []string{}})
		return
	}
	s.writeJSON(w, map[string]any{"memes": s.memes.List()})
}

func (s *Server) handleMemeAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.memes == nil {
		http.Error(w, "meme store not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path, err := s.memes.SaveFromURL(r.Context(), req.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true, "path": path})
}

func (s *Server) handleMemeDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.memes == nil {
		http.Error(w, "meme store not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.memes.Delete(req.Path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleMemeFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.memes == nil {
		http.Error(w, "meme store not enabled", http.StatusBadRequest)
		return
	}
	path := r.URL.Query().Get("path")
	resolved, err := s.memes.Resolve(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, resolved)
}

