package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
)

func TestUpdateServerRejectsSelfHostedXrayBootstrapWithoutSNI(t *testing.T) {
	db := openVIPServerAccessDB(t)
	handler := NewAdminHandler(db, nil)
	gin.SetMode(gin.TestMode)

	server := models.VPNServer{
		Name:        "Bootstrap",
		Host:        "bootstrap.example.com",
		PublicKey:   "server-public-key",
		Active:      true,
		SSHPassword: "secret",
	}
	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create server: %v", err)
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: fmt.Sprint(server.ID)}}
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/servers/"+fmt.Sprint(server.ID),
		strings.NewReader(`{"bootstrap_selfhosted_xray":true}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateServer(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "vless_server_name") {
		t.Fatalf("expected validation error to mention vless_server_name, got %s", recorder.Body.String())
	}
}

func TestSelfHostedXrayBootstrapDefaultsToXhttpRealityAuto443(t *testing.T) {
	server := &models.VPNServer{Name: "Bootstrap", Host: "203.0.113.10"}
	template := &models.VLESSServerTemplate{
		ServerName: "example.com",
	}

	options := defaultSelfHostedXrayBootstrapOptions(server, template)
	command := buildSelfHostedXrayBootstrapCommand(server, template, options)

	if options.Port != 443 {
		t.Fatalf("expected default bootstrap port 443, got %d", options.Port)
	}
	if !strings.Contains(command, `"network": "xhttp"`) {
		t.Fatalf("expected bootstrap server config to use xhttp transport")
	}
	if !strings.Contains(command, `"security": "reality"`) {
		t.Fatalf("expected bootstrap server config to use reality security")
	}
	if !strings.Contains(command, `"mode": "auto"`) {
		t.Fatalf("expected bootstrap server config to use XHTTP auto mode")
	}
	if !strings.Contains(command, `"realitySettings"`) {
		t.Fatalf("XHTTP Reality bootstrap must include realitySettings")
	}
	if !strings.Contains(command, `"dest":`) ||
		!strings.Contains(command, `"serverNames":`) {
		t.Fatalf("expected reality settings to contain dest and serverNames")
	}
	if !strings.Contains(command, `"destOverride": [`) ||
		!strings.Contains(command, `"quic"`) {
		t.Fatalf("expected bootstrap server config to enable inbound sniffing")
	}
	if !strings.Contains(command, `"protocol": "blackhole"`) ||
		!strings.Contains(command, `"geoip:private"`) ||
		!strings.Contains(command, `"bittorrent"`) {
		t.Fatalf("expected bootstrap server config to include abuse-prevention routing")
	}
}

func TestSelfHostedHysteriaBootstrapCommandInstallsUDP443(t *testing.T) {
	server := &models.VPNServer{Name: "Bootstrap", Host: "203.0.113.10"}
	template := &models.VLESSServerTemplate{
		ServerName:            "example.com",
		HysteriaPassword:      "hy-password",
		HysteriaObfsPassword:  "obfs-password",
		HysteriaMasqueradeURL: "https://www.microsoft.com",
	}

	options := defaultSelfHostedHysteriaOptions(server, template)
	command := buildSelfHostedHysteriaBootstrapCommand(options)

	if options.Port != 443 {
		t.Fatalf("expected default Hysteria2 port 443, got %d", options.Port)
	}
	if !strings.Contains(command, "listen: :${HYSTERIA_PORT}") {
		t.Fatalf("expected command to write Hysteria2 listen port")
	}
	if !strings.Contains(command, "iptables -C INPUT -p udp --dport") {
		t.Fatalf("expected command to open UDP firewall")
	}
	if !strings.Contains(command, "auth:") ||
		!strings.Contains(command, "type: password") ||
		!strings.Contains(command, "obfs:") ||
		!strings.Contains(command, "salamander") {
		t.Fatalf("expected command to configure password auth and salamander obfs")
	}
	if !strings.Contains(command, "port=%s") {
		t.Fatalf("expected command to print key-value output")
	}
}
