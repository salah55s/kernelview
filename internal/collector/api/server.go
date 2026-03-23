// Package api provides the REST API server for the KernelView dashboard.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kernelview/kernelview/pkg/models"
)

// DataSource provides data to the REST API.
type DataSource interface {
	// Service map
	GetServiceMap() ([]models.ServiceMapNode, []models.ServiceMapEdge, error)

	// Service detail
	GetServiceMetrics(service, namespace string, duration time.Duration) (interface{}, error)

	// Traces
	GetTraces(service string, start, end time.Time, limit int) ([]models.TraceEntry, error)

	// Anomalies & Incidents
	GetActiveAnomalies() ([]models.Anomaly, error)
	GetIncident(id string) (*models.Incident, error)
	GetIncidents(service, namespace string, limit int) ([]models.Incident, error)

	// Right-sizing
	GetRightSizingRecommendations() ([]models.RightSizingRecommendation, error)
}

// Server is the REST API server for the dashboard.
type Server struct {
	mux    *http.ServeMux
	data   DataSource
	logger *slog.Logger
}

// NewServer creates a new REST API server.
func NewServer(data DataSource, logger *slog.Logger) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		data:   data,
		logger: logger,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Service map
	s.mux.HandleFunc("GET /api/v1/service-map", s.handleServiceMap)

	// Service detail
	s.mux.HandleFunc("GET /api/v1/services/{service}/metrics", s.handleServiceMetrics)

	// Traces
	s.mux.HandleFunc("GET /api/v1/traces", s.handleTraces)

	// Anomalies
	s.mux.HandleFunc("GET /api/v1/anomalies", s.handleAnomalies)

	// Incidents
	s.mux.HandleFunc("GET /api/v1/incidents", s.handleIncidents)
	s.mux.HandleFunc("GET /api/v1/incidents/{id}", s.handleIncidentDetail)

	// Right-sizing
	s.mux.HandleFunc("GET /api/v1/right-sizing", s.handleRightSizing)

	// Health
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /readyz", s.handleReadyz)
}

// Start begins serving on the given address.
func (s *Server) Start(addr string) error {
	server := &http.Server{
		Addr:         addr,
		Handler:      s.corsMiddleware(s.mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("REST API server listening", "addr", addr)
	return server.ListenAndServe()
}

// --- Handlers ---

func (s *Server) handleServiceMap(w http.ResponseWriter, r *http.Request) {
	nodes, edges, err := s.data.GetServiceMap()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	})
}

func (s *Server) handleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	service := r.PathValue("service")
	namespace := r.URL.Query().Get("namespace")
	duration := parseDuration(r.URL.Query().Get("duration"), 1*time.Hour)

	metrics, err := s.data.GetServiceMetrics(service, namespace, duration)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, metrics)
}

func (s *Server) handleTraces(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	duration := parseDuration(r.URL.Query().Get("duration"), 1*time.Hour)
	limit := parseIntParam(r.URL.Query().Get("limit"), 200)

	end := time.Now()
	start := end.Add(-duration)

	traces, err := s.data.GetTraces(service, start, end, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, traces)
}

func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := s.data.GetActiveAnomalies()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, anomalies)
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	namespace := r.URL.Query().Get("namespace")
	limit := parseIntParam(r.URL.Query().Get("limit"), 50)

	incidents, err := s.data.GetIncidents(service, namespace, limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, incidents)
}

func (s *Server) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	incident, err := s.data.GetIncident(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err)
		return
	}
	s.writeJSON(w, incident)
}

func (s *Server) handleRightSizing(w http.ResponseWriter, r *http.Request) {
	recs, err := s.data.GetRightSizingRecommendations()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, recs)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, map[string]string{"status": "ready"})
}

// --- Helpers ---

func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to write JSON response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var v int
	fmt.Sscanf(s, "%d", &v)
	if v <= 0 {
		return defaultVal
	}
	return v
}
