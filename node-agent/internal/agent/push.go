package agent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type PushConfig struct {
	BackendURL       string
	NodeID           string
	Version          string
	Commit           string
	DockerBin        string
	PrivateKey       ed25519.PrivateKey
	Interval         time.Duration
	SnapshotInterval time.Duration
}

type PushWorker struct {
	cfg             PushConfig
	snapshotBuilder *SnapshotBuilder
	updateManager   *UpdateManager
	httpClient      *http.Client
	started         time.Time
	lastSnapshotHash string
	lastSnapshotAt   time.Time
}

func NewPushWorker(cfg PushConfig, snapshotBuilder *SnapshotBuilder, updateManager *UpdateManager) *PushWorker {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Minute
	}
	if cfg.SnapshotInterval <= 0 {
		cfg.SnapshotInterval = 10 * time.Minute
	}
	if cfg.DockerBin == "" {
		cfg.DockerBin = "docker"
	}
	return &PushWorker{
		cfg:             cfg,
		snapshotBuilder: snapshotBuilder,
		updateManager:   updateManager,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		started:         time.Now().UTC(),
	}
}

func ParsePrivateKey(raw string) (ed25519.PrivateKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, fmt.Errorf("private key must be %d-byte seed or %d-byte private key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func (w *PushWorker) Run(ctx context.Context) {
	if strings.TrimSpace(w.cfg.BackendURL) == "" || len(w.cfg.PrivateKey) != ed25519.PrivateKeySize {
		return
	}
	w.pushOnce(ctx)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pushOnce(ctx)
		}
	}
}

func (w *PushWorker) pushOnce(ctx context.Context) {
	if err := w.pushHeartbeat(ctx); err != nil {
		log.Printf("agent push heartbeat failed: %v", err)
	}
	if err := w.pushSnapshotIfNeeded(ctx); err != nil {
		log.Printf("agent push snapshot failed: %v", err)
	}
}

func (w *PushWorker) pushHeartbeat(ctx context.Context) error {
	state := w.updateManager.Status()
	payload := map[string]interface{}{
		"node_id":          w.cfg.NodeID,
		"version":          w.cfg.Version,
		"commit":           w.cfg.Commit,
		"uptime_seconds":   int64(time.Since(w.started).Seconds()),
		"docker_available": exec.Command(w.cfg.DockerBin, "version", "--format", "{{.Client.Version}}").Run() == nil,
		"active_digest":    state.ActiveDigest,
		"previous_digest":  state.PreviousDigest,
		"update_status":    state.Status,
		"update_error":     state.LastError,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return w.postSigned(ctx, "/node-agent/heartbeat", "application/json", body, nil)
}

func (w *PushWorker) pushSnapshotIfNeeded(ctx context.Context) error {
	packed, manifest, err := w.snapshotBuilder.Pack(ctx)
	if err != nil {
		return err
	}
	if manifest.ContentHash == w.lastSnapshotHash && time.Since(w.lastSnapshotAt) < w.cfg.SnapshotInterval {
		return nil
	}
	headers := map[string]string{
		"X-FBLink-Node-ID":        w.cfg.NodeID,
		"X-FBLink-Snapshot-Hash": manifest.ContentHash,
	}
	if err := w.postSigned(ctx, "/node-agent/snapshot", "application/gzip", packed, headers); err != nil {
		return err
	}
	w.lastSnapshotHash = manifest.ContentHash
	w.lastSnapshotAt = time.Now().UTC()
	return nil
}

func (w *PushWorker) postSigned(ctx context.Context, path, contentType string, body []byte, extraHeaders map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(w.cfg.BackendURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce, err := pushNonce()
	if err != nil {
		return err
	}
	signature := ed25519.Sign(w.cfg.PrivateKey, canonicalRequest(http.MethodPost, "/api/v1"+path, timestamp, nonce, body))
	req.Header.Set(signatureTimestampHeader, timestamp)
	req.Header.Set(signatureNonceHeader, nonce)
	req.Header.Set(signatureHeader, base64.StdEncoding.EncodeToString(signature))
	req.Header.Set("Content-Type", contentType)
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("backend returned %s", resp.Status)
	}
	return nil
}

func pushNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
