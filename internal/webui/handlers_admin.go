package webui

import (
	"encoding/json"
	"io"
	"net/http"
)

func (s *Server) handleAdminClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.mem == nil {
		http.Error(w, "memory not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		GroupID int64 `json:"group_id"`
		UserID  int64 `json:"user_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	key := req.GroupID
	if key == 0 {
		key = req.UserID
	}
	if key == 0 {
		http.Error(w, "group_id or user_id required", http.StatusBadRequest)
		return
	}
	s.mem.ClearKey(key)
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.summarizer == nil {
		http.Error(w, "summarizer not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		GroupID int64 `json:"group_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.GroupID == 0 {
		http.Error(w, "group_id required", http.StatusBadRequest)
		return
	}
	s.summarizer.SummarizeGroup(r.Context(), req.GroupID)
	s.writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleAdminSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.bot == nil {
		http.Error(w, "bot not enabled", http.StatusBadRequest)
		return
	}
	var req struct {
		GroupID int64  `json:"group_id"`
		UserID  int64  `json:"user_id"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, "message required", http.StatusBadRequest)
		return
	}
	var err error
	if req.GroupID != 0 {
		err = s.bot.SendGroupMessage(r.Context(), req.GroupID, req.Message)
	} else if req.UserID != 0 {
		err = s.bot.SendPrivateMessage(r.Context(), req.UserID, req.Message)
	} else {
		http.Error(w, "group_id or user_id required", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, map[string]any{"ok": true})
}
