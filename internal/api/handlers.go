package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// Server encapsulates the HTTP handler and routing for the debpub repository browser.
type Server struct {
	manager *RepositoryManager
	mux     *http.ServeMux
}

// NewServer initializes a new Server with API and static UI routes.
func NewServer(manager *RepositoryManager) *Server {
	s := &Server{
		manager: manager,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the underlying http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	// Web UI
	s.mux.HandleFunc("GET /", s.handleIndexHTML)

	// API endpoints
	s.mux.HandleFunc("GET /api/info", s.handleGetInfo)
	s.mux.HandleFunc("GET /api/packages", s.handleListPackages)
	s.mux.HandleFunc("GET /api/packages/{name}", s.handleGetPackage)
	s.mux.HandleFunc("POST /api/fetch", s.handleFetch)
}

func (s *Server) handleGetInfo(w http.ResponseWriter, r *http.Request) {
	codename := r.URL.Query().Get("codename")
	component := r.URL.Query().Get("component")

	info := s.manager.GetRepoInfo(codename, component)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleListPackages(w http.ResponseWriter, r *http.Request) {
	codename := r.URL.Query().Get("codename")
	component := r.URL.Query().Get("component")
	arch := r.URL.Query().Get("arch")
	query := r.URL.Query().Get("q")

	cards := s.manager.ListPackages(codename, component, arch, query)
	writeJSON(w, http.StatusOK, cards)
}

func (s *Server) handleGetPackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "package name is required")
		return
	}

	codename := r.URL.Query().Get("codename")
	component := r.URL.Query().Get("component")

	detail, found := s.manager.GetPackageDetail(codename, component, name)
	if !found {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	codename := r.URL.Query().Get("codename")
	component := r.URL.Query().Get("component")

	if err := s.manager.SyncIndexes(r.Context(), codename, component); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	info := s.manager.GetRepoInfo(codename, component)
	resp := FetchResponse{
		Success:         true,
		Message:         "Repository indexes synchronized successfully",
		Codename:        info.Codename,
		Component:       info.Component,
		PackagesCount:   info.TotalPackages,
		SyncedAt:        info.LastSyncedTime,
		ExecutionTimeMs: time.Since(start).Milliseconds(),
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleIndexHTML(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(IndexHTML))
}

func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error":   true,
		"message": message,
	})
}
