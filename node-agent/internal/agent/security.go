package agent

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	signatureHeader          = "X-FBLink-Signature"
	signatureTimestampHeader = "X-FBLink-Timestamp"
	signatureNonceHeader     = "X-FBLink-Nonce"
)

type Verifier struct {
	publicKey ed25519.PublicKey
	maxSkew   time.Duration
	nonces    map[string]time.Time
	mu        sync.Mutex
	now       func() time.Time
}

func NewVerifier(publicKey ed25519.PublicKey, maxSkew time.Duration) *Verifier {
	return &Verifier{
		publicKey: publicKey,
		maxSkew:   maxSkew,
		nonces:    map[string]time.Time{},
		now:       time.Now,
	}
}

func ParsePublicKey(raw string) (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(key), nil
}

func (v *Verifier) Verify(r *http.Request, body []byte) error {
	if v == nil {
		return errors.New("request verifier is not configured")
	}
	timestamp := r.Header.Get(signatureTimestampHeader)
	nonce := r.Header.Get(signatureNonceHeader)
	signatureB64 := r.Header.Get(signatureHeader)
	if timestamp == "" || nonce == "" || signatureB64 == "" {
		return errors.New("missing signature headers")
	}

	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if skew := v.now().UTC().Sub(ts.UTC()); skew > v.maxSkew || skew < -v.maxSkew {
		return errors.New("signature timestamp outside allowed skew")
	}

	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}
	if !ed25519.Verify(v.publicKey, canonicalRequest(r.Method, r.URL.Path, timestamp, nonce, body), signature) {
		return errors.New("signature verification failed")
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.pruneLocked()
	if _, exists := v.nonces[nonce]; exists {
		return errors.New("replayed nonce")
	}
	v.nonces[nonce] = ts.UTC()
	return nil
}

func (v *Verifier) pruneLocked() {
	cutoff := v.now().UTC().Add(-v.maxSkew)
	for nonce, ts := range v.nonces {
		if ts.Before(cutoff) {
			delete(v.nonces, nonce)
		}
	}
}

func canonicalRequest(method, path, timestamp, nonce string, body []byte) []byte {
	sum := sha256.Sum256(body)
	return []byte(method + "\n" + path + "\n" + timestamp + "\n" + nonce + "\n" + hex.EncodeToString(sum[:]))
}
