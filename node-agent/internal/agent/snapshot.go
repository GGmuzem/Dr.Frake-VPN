package agent

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type SnapshotConfig struct {
	NodeID          string
	DockerBin       string
	XrayContainer   string
	AWGContainer    string
	AWGInterface    string
	PiHoleContainer string
}

type SnapshotManifest struct {
	NodeID      string                 `json:"node_id"`
	CreatedAt   time.Time              `json:"created_at"`
	ContentHash string                 `json:"content_hash"`
	Files       []SnapshotManifestFile `json:"files"`
}

type SnapshotManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

type SnapshotBuilder struct {
	cfg    SnapshotConfig
	runner Runner
	now    func() time.Time
}

func NewSnapshotBuilder(cfg SnapshotConfig, runner Runner) *SnapshotBuilder {
	if cfg.DockerBin == "" {
		cfg.DockerBin = "docker"
	}
	if cfg.XrayContainer == "" {
		cfg.XrayContainer = "amnezia-xray"
	}
	if cfg.AWGContainer == "" {
		cfg.AWGContainer = "amnezia-awg2"
	}
	if cfg.AWGInterface == "" {
		cfg.AWGInterface = "awg0"
	}
	return &SnapshotBuilder{cfg: cfg, runner: runner, now: time.Now}
}

func (b *SnapshotBuilder) Pack(ctx context.Context) ([]byte, SnapshotManifest, error) {
	files := map[string][]byte{}
	readOptional := func(snapshotPath, container, shell string) {
		out, err := b.runner.Run(ctx, b.cfg.DockerBin, "exec", container, "sh", "-lc", shell)
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			files[snapshotPath] = bytes.TrimSpace(out)
		}
	}

	xrayPrefix := "cat /opt/amnezia/xray/%s 2>/dev/null || cat /opt/fblink/xray/%s 2>/dev/null"
	for _, name := range []string{
		"server.json",
		"xray_public.key",
		"xray_short_id.key",
		"xray_uuid.key",
		"xray_mldsa65_verify.key",
	} {
		readOptional(path.Join("xray", name), b.cfg.XrayContainer, fmt.Sprintf(xrayPrefix, name, name))
	}
	readOptional(path.Join("awg", b.cfg.AWGInterface+".conf"), b.cfg.AWGContainer, fmt.Sprintf("cat /opt/amnezia/awg/%s.conf 2>/dev/null || cat /opt/fblink/awg/%s.conf 2>/dev/null", b.cfg.AWGInterface, b.cfg.AWGInterface))
	if strings.TrimSpace(b.cfg.PiHoleContainer) != "" {
		readOptional("pihole/metadata.txt", b.cfg.PiHoleContainer, "test -f /etc/pihole/gravity.db && echo gravity-db-present || true")
	}

	manifest := SnapshotManifest{
		NodeID:    b.cfg.NodeID,
		CreatedAt: b.now().UTC(),
		Files:     make([]SnapshotManifestFile, 0, len(files)),
	}
	hash := sha256.New()
	for filePath, content := range files {
		sum := sha256.Sum256(content)
		manifest.Files = append(manifest.Files, SnapshotManifestFile{
			Path:   filePath,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   len(content),
		})
		_, _ = hash.Write([]byte(filePath))
		_, _ = hash.Write(sum[:])
	}
	manifest.ContentHash = hex.EncodeToString(hash.Sum(nil))

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, manifest, err
	}
	files["manifest.json"] = manifestBytes

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for filePath, content := range files {
		if err := addTarFile(tw, filePath, content); err != nil {
			return nil, manifest, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, manifest, err
	}
	if err := gz.Close(); err != nil {
		return nil, manifest, err
	}
	return buf.Bytes(), manifest, nil
}

func addTarFile(tw *tar.Writer, name string, content []byte) error {
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return fmt.Errorf("unsafe snapshot path %q", name)
	}
	h := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(content)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := io.Copy(tw, bytes.NewReader(content))
	return err
}
