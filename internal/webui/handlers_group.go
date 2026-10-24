package webui

import (
	"net/http"
	"strconv"

	"icarosu/internal/store"
)

func (s *Server) handleGroupStyles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		s.writeJSON(w, map[string]any{"expressions": []store.StyleExpressionRecord{}})
		return
	}
	records, err := s.store.LoadStyleExpressions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, map[string]any{"expressions": records})
}

func (s *Server) handleGroupJargons(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		s.writeJSON(w, map[string]any{"jargons": []store.JargonRecord{}})
		return
	}
	groupID := int64(0)
	if v := r.URL.Query().Get("group_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			groupID = n
		}
	}
	records, err := s.store.LoadJargonsByGroup(groupID, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, map[string]any{"jargons": records})
}

func (s *Server) handleGroupProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.store == nil {
		s.writeJSON(w, map[string]any{"profiles": []store.UserProfileRecord{}})
		return
	}
	groupID := int64(0)
	if v := r.URL.Query().Get("group_id"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			groupID = n
		}
	}
	records, err := s.store.LoadUserProfiles(groupID, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, map[string]any{"profiles": records})
}