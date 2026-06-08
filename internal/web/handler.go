package web

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"

	"aigateway/internal/config"
	"aigateway/internal/store"
)

//go:embed templates/*
var templatesFS embed.FS

// Handler serves the admin UI and log API.
type Handler struct {
	store  *store.Store
	config *config.Config
	tmpl   *template.Template
}

// NewHandler creates a new web handler.
func NewHandler(s *store.Store, cfg *config.Config) (*Handler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/logs.html")
	if err != nil {
		return nil, err
	}
	return &Handler{store: s, config: cfg, tmpl: tmpl}, nil
}

// ServeHTTP routes between the admin page and the JSON API.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/admin", "/admin/":
		h.serveAdminPage(w, r)
	case "/api/logs":
		h.serveLogsAPI(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) serveAdminPage(w http.ResponseWriter, r *http.Request) {
	routes := make([]string, 0, len(h.config.Routes))
	for _, rc := range h.config.Routes {
		routes = append(routes, rc.Prefix)
	}
	data := map[string]any{
		"Routes": routes,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) serveLogsAPI(w http.ResponseWriter, r *http.Request) {
	route := r.URL.Query().Get("route")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	result, err := h.store.QueryLogs(route, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
