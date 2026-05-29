package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewRateLimiter(t *testing.T) {
	limit := 5
	window := time.Second * 10

	rl := NewRateLimiter(limit, window)

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	if rl.limit != limit {
		t.Errorf("Expected limit %d, got %d", limit, rl.limit)
	}

	if rl.window != window {
		t.Errorf("Expected window %v, got %v", window, rl.window)
	}

	if rl.visitors == nil {
		t.Error("Expected visitors map to be initialized")
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	limit := 3
	window := time.Minute

	rl := NewRateLimiter(limit, window)
	ip := "192.168.1.1"

	// Initial request should be allowed
	if !rl.Allow(ip) {
		t.Errorf("Expected first request from %s to be allowed", ip)
	}

	// Two more requests should be allowed
	rl.Allow(ip)
	rl.Allow(ip)

	// Fourth request should be blocked
	if rl.Allow(ip) {
		t.Errorf("Expected fourth request from %s to be blocked", ip)
	}

	// Different IP should be allowed
	if !rl.Allow("10.0.0.1") {
		t.Error("Expected request from new IP to be allowed")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limit := 5
	window := time.Millisecond * 50

	rl := NewRateLimiter(limit, window)
	ip := "192.168.1.1"

	// Add a visitor
	rl.Allow(ip)

	// Wait for cleanup to run
	time.Sleep(window * 2)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if _, exists := rl.visitors[ip]; exists {
		t.Errorf("Expected visitor %s to be cleaned up", ip)
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(2, time.Minute)

	r := gin.New()
	r.Use(RateLimit(rl))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Create helper function to make requests
	makeRequest := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345" // Gin uses this for ClientIP
		r.ServeHTTP(w, req)
		return w
	}

	// Request 1: Allowed
	w1 := makeRequest()
	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w1.Code)
	}

	// Request 2: Allowed
	w2 := makeRequest()
	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	// Request 3: Blocked
	w3 := makeRequest()
	if w3.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w3.Code)
	}
}
