package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os/exec"
	"time"
)

type ServerConfig struct {
	Version         string
	Commit          string
	NodeID          string
	ListenAddr      string
	StatePath       string
	DockerBin       string
	Verifier        *Verifier
	SnapshotBuilder *SnapshotBuilder
	UpdateManager   *UpdateManager
}

type Server struct {
	cfg      ServerConfig
	started  time.Time
	mux      *http.ServeMux
	stateOut func(UpdateState) error
}

func NewServer(cfg ServerConfig) *Server {
	s := &Server{cfg: cfg, started: time.Now().UTC(), mux: http.NewServeMux(), stateOut: func(state UpdateState) error {
		return SaveUpdateState(cfg.StatePath, state)
	}}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.health)
	s.mux.HandleFunc("/snapshot", s.signed(s.snapshot))
	s.mux.HandleFunc("/updates/apply", s.signed(s.applyUpdate))
	s.mux.HandleFunc("/updates/status", s.signed(s.updateStatus))
	s.mux.HandleFunc("/updates/rollback", s.signed(s.rollbackUpdate))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dockerBin := s.cfg.DockerBin
	if dockerBin == "" {
		dockerBin = "docker"
	}
	dockerAvailable := exec.Command(dockerBin, "version", "--format", "{{.Client.Version}}").Run() == nil
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":          s.cfg.Version,
		"commit":           s.cfg.Commit,
		"node_id":          s.cfg.NodeID,
		"uptime_seconds":   int64(time.Since(s.started).Seconds()),
		"docker_available": dockerAvailable,
	})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request, _ []byte) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	packed, manifest, err := s.cfg.SnapshotBuilder.Pack(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("X-FBLink-Snapshot-Hash", manifest.ContentHash)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(packed)
}

func (s *Server) applyUpdate(w http.ResponseWriter, r *http.Request, body []byte) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ImageDigest string `json:"image_digest"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	state := s.cfg.UpdateManager.Apply(r.Context(), req.ImageDigest)
	_ = s.stateOut(state)
	if state.Status == UpdateStatusFailed {
		writeJSON(w, http.StatusBadRequest, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request, _ []byte) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.UpdateManager.Status())
}

func (s *Server) rollbackUpdate(w http.ResponseWriter, r *http.Request, _ []byte) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := s.cfg.UpdateManager.Rollback(r.Context())
	_ = s.stateOut(state)
	if state.Status == UpdateStatusFailed {
		writeJSON(w, http.StatusBadRequest, state)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) signed(next func(http.ResponseWriter, *http.Request, []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := []byte{}
		if r.Body != nil {
			defer r.Body.Close()
			var err error
			body, err = ioReadAll(r.Context(), r.Body)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}
		if err := s.cfg.Verifier.Verify(r, body); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		next(w, r, body)
	}
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type reader interface {
	Read([]byte) (int, error)
}

func ioReadAll(ctx context.Context, r reader) ([]byte, error) {
	type result struct {
		body []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		body, err := io.ReadAll(r)
		ch <- result{body: body, err: err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-ch:
		return result.body, result.err
	}
}
