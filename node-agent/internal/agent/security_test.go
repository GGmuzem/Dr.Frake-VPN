package agent

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"testing"
	"time"
)

func signedRequest(t *testing.T, private ed25519.PrivateKey, method, path string, body []byte, ts time.Time, nonce string) *http.Request {
	t.Helper()

	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	timestamp := ts.UTC().Format(time.RFC3339)
	req.Header.Set(signatureTimestampHeader, timestamp)
	req.Header.Set(signatureNonceHeader, nonce)
	req.Header.Set(signatureHeader, base64.StdEncoding.EncodeToString(ed25519.Sign(private, canonicalRequest(method, req.URL.Path, timestamp, nonce, body))))
	return req
}

func TestVerifierAcceptsValidSignature(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	verifier := NewVerifier(public, 5*time.Minute)
	body := []byte(`{"image_digest":"repo/app@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	req := signedRequest(t, private, http.MethodPost, "/updates/apply", body, time.Now(), "nonce-1")

	if err := verifier.Verify(req, body); err != nil {
		t.Fatalf("expected valid signature: %v", err)
	}
}

func TestVerifierRejectsTamperedBody(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	verifier := NewVerifier(public, 5*time.Minute)
	req := signedRequest(t, private, http.MethodPost, "/updates/apply", []byte(`{"ok":true}`), time.Now(), "nonce-1")

	if err := verifier.Verify(req, []byte(`{"ok":false}`)); err == nil {
		t.Fatal("expected tampered body to fail verification")
	}
}

func TestVerifierRejectsExpiredTimestamp(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	verifier := NewVerifier(public, 5*time.Minute)
	req := signedRequest(t, private, http.MethodGet, "/snapshot", nil, time.Now().Add(-10*time.Minute), "nonce-1")

	if err := verifier.Verify(req, nil); err == nil {
		t.Fatal("expected expired timestamp to fail verification")
	}
}

func TestVerifierRejectsReplayedNonce(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	verifier := NewVerifier(public, 5*time.Minute)
	req1 := signedRequest(t, private, http.MethodGet, "/snapshot", nil, time.Now(), "nonce-1")
	req2 := signedRequest(t, private, http.MethodGet, "/snapshot", nil, time.Now(), "nonce-1")

	if err := verifier.Verify(req1, nil); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	if err := verifier.Verify(req2, nil); err == nil {
		t.Fatal("expected replayed nonce to fail verification")
	}
}
