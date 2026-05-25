package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	agentSignatureHeader          = "X-FBLink-Signature"
	agentSignatureTimestampHeader = "X-FBLink-Timestamp"
	agentSignatureNonceHeader     = "X-FBLink-Nonce"
)

type nodeAgentClient struct {
	baseURL    string
	privateKey ed25519.PrivateKey
	httpClient *http.Client
}

type nodeAgentHealth struct {
	Version         string `json:"version"`
	Commit          string `json:"commit"`
	NodeID          string `json:"node_id"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
	DockerAvailable bool   `json:"docker_available"`
}

type nodeAgentUpdateState struct {
	Status         string    `json:"status"`
	ActiveDigest   string    `json:"active_digest"`
	PreviousDigest string    `json:"previous_digest"`
	LastError      string    `json:"last_error"`
	UpdatedAt      time.Time `json:"updated_at"`
	CanRollback    bool      `json:"can_rollback"`
}

type nodeAgentSnapshot struct {
	Body        []byte
	ContentHash string
	Files       map[string][]byte
}

func newNodeAgentClient(rawURL, signingKey string) (*nodeAgentClient, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, fmt.Errorf("agent_url is not configured")
	}
	if strings.TrimSpace(signingKey) == "" {
		return nil, fmt.Errorf("AGENT_SIGNING_PRIVATE_KEY is not configured")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("agent_url must be http or https")
	}
	privateKey, err := parseAgentSigningKey(signingKey)
	if err != nil {
		return nil, err
	}
	return &nodeAgentClient{
		baseURL:    strings.TrimRight(rawURL, "/"),
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func parseAgentSigningKey(raw string) (ed25519.PrivateKey, error) {
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
		return nil, fmt.Errorf("agent signing key must be %d-byte seed or %d-byte private key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

func agentVerifyPublicKey(signingKey string) (string, error) {
	privateKey, err := parseAgentSigningKey(signingKey)
	if err != nil {
		return "", err
	}
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("failed to derive Ed25519 public key")
	}
	return base64.StdEncoding.EncodeToString(publicKey), nil
}

func (c *nodeAgentClient) Health() (nodeAgentHealth, error) {
	var health nodeAgentHealth
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return health, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return health, fmt.Errorf("agent health returned %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(&health)
	return health, err
}

func (c *nodeAgentClient) Snapshot() (nodeAgentSnapshot, error) {
	resp, err := c.doSigned(http.MethodGet, "/snapshot", nil)
	if err != nil {
		return nodeAgentSnapshot{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nodeAgentSnapshot{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nodeAgentSnapshot{}, fmt.Errorf("agent snapshot returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	files, err := readSnapshotTarGz(body)
	if err != nil {
		return nodeAgentSnapshot{}, err
	}
	return nodeAgentSnapshot{Body: body, ContentHash: resp.Header.Get("X-FBLink-Snapshot-Hash"), Files: files}, nil
}

func (c *nodeAgentClient) Update(serverID uint, imageDigest string) (nodeAgentUpdateState, error) {
	body, _ := json.Marshal(map[string]interface{}{"server_id": serverID, "image_digest": imageDigest})
	return c.updateCall("/updates/apply", body)
}

func (c *nodeAgentClient) Status() (nodeAgentUpdateState, error) {
	return c.updateCall("/updates/status", nil)
}

func (c *nodeAgentClient) Rollback() (nodeAgentUpdateState, error) {
	return c.updateCall("/updates/rollback", nil)
}

func (c *nodeAgentClient) updateCall(path string, body []byte) (nodeAgentUpdateState, error) {
	method := http.MethodGet
	if body != nil || strings.HasSuffix(path, "/rollback") {
		method = http.MethodPost
	}
	resp, err := c.doSigned(method, path, body)
	if err != nil {
		return nodeAgentUpdateState{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nodeAgentUpdateState{}, err
	}
	var state nodeAgentUpdateState
	if err := json.Unmarshal(respBody, &state); err != nil {
		return state, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return state, fmt.Errorf("agent update returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	return state, nil
}

func (c *nodeAgentClient) doSigned(method, path string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce, err := randomNonce()
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(c.privateKey, agentCanonicalRequest(method, path, timestamp, nonce, body))
	req.Header.Set(agentSignatureTimestampHeader, timestamp)
	req.Header.Set(agentSignatureNonceHeader, nonce)
	req.Header.Set(agentSignatureHeader, base64.StdEncoding.EncodeToString(signature))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func agentCanonicalRequest(method, path, timestamp, nonce string, body []byte) []byte {
	sum := sha256.Sum256(body)
	return []byte(method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(sum[:]))
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func readSnapshotTarGz(body []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	files := map[string][]byte{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if header.FileInfo().IsDir() {
			continue
		}
		if strings.Contains(header.Name, "..") || strings.HasPrefix(header.Name, "/") {
			return nil, fmt.Errorf("unsafe snapshot path %q", header.Name)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[header.Name] = content
	}
}
