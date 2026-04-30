package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/prods/nvimon/internal/collector"
)

type Server struct {
	collector collector.Collector
	authToken string
}

func NewServer(c collector.Collector, authToken string) *Server {
	return &Server{
		collector: c,
		authToken: authToken,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/v1/snapshot", s.handleSnapshot)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if err := s.authorize(r); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	snapshot, err := s.collector.Collect(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) authorize(r *http.Request) error {
	if s.authToken == "" {
		return nil
	}

	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return errors.New("missing authorization")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return errors.New("invalid authorization scheme")
	}

	if strings.TrimPrefix(header, prefix) != s.authToken {
		return errors.New("invalid token")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func CollectOnce(ctx context.Context, c collector.Collector) ([]byte, error) {
	snapshot, err := c.Collect(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(snapshot)
}
