package webui

import (
	"fmt"
	"net/http"
	"time"
)
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "bot_uptime_seconds %d\n", int64(time.Since(s.startedAt).Seconds()))
	_, _ = fmt.Fprintf(w, "bot_info{addr=%q,auth_enabled=%t} 1\n", s.manager.Get().WebUI.Addr, s.manager.Get().WebUI.Token != "")
	if s.llm != nil {
		stats := s.llm.Stats()
		_, _ = fmt.Fprintf(w, "llm_requests_total %d\n", stats.Requests)
		_, _ = fmt.Fprintf(w, "llm_tokens_total %d\n", stats.Tokens)
		_, _ = fmt.Fprintf(w, "llm_errors_total %d\n", stats.Errors)
	}
	if s.store != nil {
		if n, err := s.store.CountMessages(); err == nil {
			_, _ = fmt.Fprintf(w, "messages_total %d\n", n)
		}
		if n, err := s.store.CountMemories(); err == nil {
			_, _ = fmt.Fprintf(w, "memories_total %d\n", n)
		}
		if n, err := s.store.CountMembers(); err == nil {
			_, _ = fmt.Fprintf(w, "members_total %d\n", n)
		}
		if groups, err := s.store.GroupIDs(); err == nil {
			_, _ = fmt.Fprintf(w, "groups_total %d\n", len(groups))
		}
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := map[string]any{
		"running":      true,
		"dry_run":      s.manager.Get().DryRun,
		"webui_addr":   s.manager.Get().WebUI.Addr,
		"auth_enabled": s.manager.Get().WebUI.Token != "",
	}
	if s.store != nil {
		groups, _ := s.store.GroupIDs()
		messages, _ := s.store.CountMessages()
		memories, _ := s.store.CountMemories()
		members, _ := s.store.CountMembers()
		status["group_ids"] = groups
		status["groups"] = len(groups)
		status["messages"] = messages
		status["memories"] = memories
		status["members"] = members
	}
	if s.attention != nil {
		status["attention"] = s.attention.Scores()
	}
	if s.llm != nil {
		status["llm"] = s.llm.Stats()
	}
	s.writeJSON(w, status)
}

