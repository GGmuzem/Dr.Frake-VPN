package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"vpn-backend/internal/models"
)

func TestBuildVLESSConfigPinsVIPDNSOverProxy(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:     "138.124.101.69",
		Port:        8443,
		ServerName:  "www.googletagmanager.com",
		PublicKey:   "pub",
		ShortID:     "short",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
	}
	profiles := []models.RoutingProfile{
		{
			Enabled:            true,
			Action:             models.RoutingProfileProxy,
			DomainSuffixesJSON: `["youtube.com"]`,
		},
	}

	config := buildVLESSConfig("client-id", server, template, profiles, vipDNSConfig{Primary: vipDNSAdBlockIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	if xrayConfig == "" {
		t.Fatalf("expected serialized xray config")
	}

	parsed := parseConfigJSON(t, xrayConfig)
	rules := parsed["routing"].(map[string]interface{})["rules"].([]interface{})
	if len(rules) == 0 {
		t.Fatalf("expected routing rules")
	}

	firstRule := rules[0].(map[string]interface{})
	if firstRule["outboundTag"] != xrayBlockTag || firstRule["network"] != "udp" || firstRule["port"] != "443" {
		t.Fatalf("expected first rule to block QUIC udp/443, got %v", firstRule)
	}

	dnsRule := rules[1].(map[string]interface{})
	if dnsRule["outboundTag"] != xrayProxyTag {
		t.Fatalf("expected DNS rule outboundTag=%q, got %v", xrayProxyTag, dnsRule["outboundTag"])
	}

	ipRules := dnsRule["ip"].([]interface{})
	if len(ipRules) < 2 {
		t.Fatalf("expected vip dns proxy rule to contain both internal dns IPs, got %v", ipRules)
	}
}

func TestBuildVLESSConfigAcceptsLegacyPlainTextRules(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:     "138.124.101.69",
		Port:        8443,
		ServerName:  "www.googletagmanager.com",
		PublicKey:   "pub",
		ShortID:     "short",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
	}
	profiles := []models.RoutingProfile{
		{
			Enabled:            true,
			Action:             models.RoutingProfileProxy,
			DomainsJSON:        "chatgpt.com\nclaude.ai",
			DomainSuffixesJSON: ".openai.com, .anthropic.com",
		},
	}

	config := buildVLESSConfig("client-id", server, template, profiles, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	if xrayConfig == "" {
		t.Fatalf("expected serialized xray config")
	}

	parsed := parseConfigJSON(t, xrayConfig)
	rules := parsed["routing"].(map[string]interface{})["rules"].([]interface{})
	if len(rules) == 0 {
		t.Fatalf("expected routing rules for legacy plain-text profile fields")
	}

	hasAIProxyRule := false
	for _, ruleValue := range rules {
		rule := ruleValue.(map[string]interface{})
		if rule["outboundTag"] != xrayProxyTag {
			continue
		}
		domains, ok := rule["domain"].([]interface{})
		if !ok || len(domains) == 0 {
			continue
		}
		if containsInterfaceString(domains, "full:chatgpt.com") &&
			containsInterfaceString(domains, "full:claude.ai") &&
			containsInterfaceString(domains, "domain:openai.com") &&
			containsInterfaceString(domains, "domain:anthropic.com") {
			hasAIProxyRule = true
			break
		}
	}

	if !hasAIProxyRule {
		t.Fatalf("expected proxy routing rule with AI domains from legacy plain text fields, got rules=%v", rules)
	}
}

func TestBuildVLESSConfigIgnoresSystemTemplateProfiles(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:     "138.124.101.69",
		Port:        8443,
		ServerName:  "www.googletagmanager.com",
		PublicKey:   "pub",
		ShortID:     "short",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
	}
	profiles := []models.RoutingProfile{
		{
			Kind:               models.RoutingProfileSystem,
			Enabled:            true,
			Action:             models.RoutingProfileProxy,
			DomainSuffixesJSON: `["youtube.com"]`,
		},
		{
			Kind:               models.RoutingProfileCustom,
			Enabled:            true,
			Action:             models.RoutingProfileProxy,
			DomainSuffixesJSON: `["chatgpt.com"]`,
		},
	}

	config := buildVLESSConfig("client-id", server, template, profiles, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	rules := parsed["routing"].(map[string]interface{})["rules"].([]interface{})

	containsYouTubeSystemDomain := false
	containsChatGPTCustomDomain := false
	for _, ruleValue := range rules {
		rule := ruleValue.(map[string]interface{})
		domains, ok := rule["domain"].([]interface{})
		if !ok {
			continue
		}
		if containsInterfaceString(domains, "domain:youtube.com") {
			containsYouTubeSystemDomain = true
		}
		if containsInterfaceString(domains, "domain:chatgpt.com") {
			containsChatGPTCustomDomain = true
		}
	}

	if containsYouTubeSystemDomain {
		t.Fatalf("system template routing rules must not be applied at runtime, got rules=%v", rules)
	}
	if !containsChatGPTCustomDomain {
		t.Fatalf("custom routing rules must still be applied, got rules=%v", rules)
	}
}

func TestBuildVLESSConfigUsesSystemProfilesFallbackWhenNoCustomEnabled(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:     "138.124.101.69",
		Port:        8443,
		ServerName:  "www.googletagmanager.com",
		PublicKey:   "pub",
		ShortID:     "short",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
	}
	profiles := []models.RoutingProfile{
		{
			Kind:               models.RoutingProfileSystem,
			Enabled:            true,
			Action:             models.RoutingProfileProxy,
			DomainSuffixesJSON: `["youtube.com"]`,
		},
	}

	config := buildVLESSConfig("client-id", server, template, profiles, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	rules := parsed["routing"].(map[string]interface{})["rules"].([]interface{})

	containsYouTubeSystemDomain := false
	for _, ruleValue := range rules {
		rule := ruleValue.(map[string]interface{})
		domains, ok := rule["domain"].([]interface{})
		if !ok {
			continue
		}
		if containsInterfaceString(domains, "domain:youtube.com") {
			containsYouTubeSystemDomain = true
			break
		}
	}

	if !containsYouTubeSystemDomain {
		t.Fatalf("expected system profile rules to be used as fallback when no custom profile is enabled, got rules=%v", rules)
	}
}

func TestBuildVLESSConfigUsesGrpcTransportSettings(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:         "138.124.101.69",
		Port:            8443,
		ServerName:      "example.com",
		PublicKey:       "pub",
		ShortID:         "short",
		Fingerprint:     "chrome",
		Flow:            "",
		Network:         "grpc",
		Security:        "reality",
		GrpcServiceName: "api.v1.VideoDownload",
		GrpcAuthority:   "grpc.example.com",
		GrpcMultiMode:   true,
	}

	config := buildVLESSConfig("client-id", server, template, nil, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	outbound := parsed["outbounds"].([]interface{})[0].(map[string]interface{})
	streamSettings := outbound["streamSettings"].(map[string]interface{})

	if streamSettings["network"] != "grpc" {
		t.Fatalf("expected grpc network, got %v", streamSettings["network"])
	}
	if _, ok := streamSettings["tcpSettings"]; ok {
		t.Fatalf("grpc streamSettings must not include tcpSettings: %v", streamSettings)
	}

	grpcSettings := streamSettings["grpcSettings"].(map[string]interface{})
	if grpcSettings["serviceName"] != "api.v1.VideoDownload" {
		t.Fatalf("unexpected grpc serviceName: %v", grpcSettings["serviceName"])
	}
	if grpcSettings["authority"] != "grpc.example.com" {
		t.Fatalf("unexpected grpc authority: %v", grpcSettings["authority"])
	}
	if grpcSettings["multiMode"] != true {
		t.Fatalf("expected grpc multiMode=true, got %v", grpcSettings["multiMode"])
	}
}

func TestBuildVLESSConfigKeepsTCPTransportSettings(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:     "138.124.101.69",
		Port:        8443,
		ServerName:  "example.com",
		PublicKey:   "pub",
		ShortID:     "short",
		Fingerprint: "chrome",
		Flow:        "xtls-rprx-vision",
		Network:     "tcp",
		Security:    "reality",
	}

	config := buildVLESSConfig("client-id", server, template, nil, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	outbound := parsed["outbounds"].([]interface{})[0].(map[string]interface{})
	streamSettings := outbound["streamSettings"].(map[string]interface{})

	if streamSettings["network"] != "tcp" {
		t.Fatalf("expected tcp network, got %v", streamSettings["network"])
	}
	if _, ok := streamSettings["tcpSettings"]; !ok {
		t.Fatalf("tcp streamSettings must include tcpSettings: %v", streamSettings)
	}
	if _, ok := streamSettings["grpcSettings"]; ok {
		t.Fatalf("tcp streamSettings must not include grpcSettings: %v", streamSettings)
	}
}

func parseConfigJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	return parsed
}

func containsInterfaceString(values []interface{}, needle string) bool {
	for _, value := range values {
		if fmt.Sprint(value) == needle {
			return true
		}
	}
	return false
}

func TestBuildVLESSConfigUsesXhttpTransportSettings(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:         "138.124.101.69",
		Port:            8443,
		ServerName:      "example.com",
		PublicKey:       "pub",
		ShortID:         "short",
		Fingerprint:     "chrome",
		Flow:            "xtls-rprx-vision",
		Network:         "xhttp",
		Security:        "tls",
		GrpcServiceName: "/assets/7d91f0e4",
	}

	config := buildVLESSConfig("client-id", server, template, nil, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	outbound := parsed["outbounds"].([]interface{})[0].(map[string]interface{})
	streamSettings := outbound["streamSettings"].(map[string]interface{})

	if streamSettings["network"] != "xhttp" {
		t.Fatalf("expected xhttp network, got %v", streamSettings["network"])
	}
	if _, ok := streamSettings["tcpSettings"]; ok {
		t.Fatalf("xhttp streamSettings must not include tcpSettings")
	}
	if _, ok := streamSettings["grpcSettings"]; ok {
		t.Fatalf("xhttp streamSettings must not include grpcSettings")
	}
	if _, ok := streamSettings["realitySettings"]; ok {
		t.Fatalf("xhttp tls streamSettings must not include realitySettings")
	}

	xhttpSettings := streamSettings["xhttpSettings"].(map[string]interface{})
	if xhttpSettings["path"] != "/assets/7d91f0e4" {
		t.Fatalf("unexpected xhttp path: %v", xhttpSettings["path"])
	}
	if xhttpSettings["mode"] != "auto" {
		t.Fatalf("expected xhttp mode=auto, got %v", xhttpSettings["mode"])
	}
	tlsSettings := streamSettings["tlsSettings"].(map[string]interface{})
	if tlsSettings["serverName"] != "example.com" {
		t.Fatalf("unexpected tls serverName: %v", tlsSettings["serverName"])
	}
	if !containsInterfaceString(tlsSettings["alpn"].([]interface{}), "h3") {
		t.Fatalf("expected client tlsSettings alpn to include h3, got %v", tlsSettings["alpn"])
	}

	// Verify no flow in user credentials
	user := outbound["settings"].(map[string]interface{})["vnext"].([]interface{})[0].(map[string]interface{})["users"].([]interface{})[0].(map[string]interface{})
	if flow, ok := user["flow"]; ok && flow != "" {
		t.Fatalf("expected empty flow for xhttp, got %v", flow)
	}

	// Verify buildHappVLESSURI yields correct URI with path and without flow
	uri := buildHappVLESSURI("client-id", server, template)
	if !strings.Contains(uri, "type=xhttp") {
		t.Fatalf("expected type=xhttp in URI, got %s", uri)
	}
	if !strings.Contains(uri, "path=%2Fassets%2F7d91f0e4") {
		t.Fatalf("expected path=/assets/7d91f0e4 in URI, got %s", uri)
	}
	if !strings.Contains(uri, "mode=auto") {
		t.Fatalf("expected mode=auto in URI, got %s", uri)
	}
	if !strings.Contains(uri, "alpn=h3") {
		t.Fatalf("expected alpn=h3 in URI, got %s", uri)
	}
	if strings.Contains(uri, "pbk=") || strings.Contains(uri, "sid=") {
		t.Fatalf("expected no REALITY params in XHTTP TLS URI, got %s", uri)
	}
	if strings.Contains(uri, "flow=") {
		t.Fatalf("expected no flow in VLESS URI for xhttp, got %s", uri)
	}
}

func TestBuildVLESSConfigUsesXhttpRealitySettings(t *testing.T) {
	server := &models.VPNServer{Name: "Test", Region: "AMS", CountryCode: "NL"}
	template := &models.VLESSServerTemplate{
		Address:         "138.124.101.69",
		Port:            8443,
		ServerName:      "example.com",
		PublicKey:       "pubkey123",
		ShortID:         "sid123",
		Fingerprint:     "chrome",
		Network:         "xhttp",
		Security:        "reality",
		GrpcServiceName: "/assets/7d91f0e4",
	}

	config := buildVLESSConfig("client-id", server, template, nil, vipDNSConfig{Primary: vipDNSCleanIP})
	xrayConfig := config["containers"].([]interface{})[0].(map[string]interface{})["xray"].(map[string]interface{})["last_config"].(string)
	parsed := parseConfigJSON(t, xrayConfig)
	outbound := parsed["outbounds"].([]interface{})[0].(map[string]interface{})
	streamSettings := outbound["streamSettings"].(map[string]interface{})

	if streamSettings["network"] != "xhttp" {
		t.Fatalf("expected xhttp network, got %v", streamSettings["network"])
	}
	if streamSettings["security"] != "reality" {
		t.Fatalf("expected reality security, got %v", streamSettings["security"])
	}
	if _, ok := streamSettings["tlsSettings"]; ok {
		t.Fatalf("xhttp reality streamSettings must not include tlsSettings")
	}
	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	if realitySettings["publicKey"] != "pubkey123" {
		t.Fatalf("unexpected reality publicKey: %v", realitySettings["publicKey"])
	}
	if realitySettings["shortId"] != "sid123" {
		t.Fatalf("unexpected reality shortId: %v", realitySettings["shortId"])
	}

	xhttpSettings := streamSettings["xhttpSettings"].(map[string]interface{})
	if xhttpSettings["path"] != "/assets/7d91f0e4" {
		t.Fatalf("unexpected xhttp path: %v", xhttpSettings["path"])
	}
	if xhttpSettings["mode"] != "auto" {
		t.Fatalf("expected xhttp mode=auto, got %v", xhttpSettings["mode"])
	}

	uri := buildHappVLESSURI("client-id", server, template)
	if !strings.Contains(uri, "security=reality") {
		t.Fatalf("expected security=reality in URI, got %s", uri)
	}
	if !strings.Contains(uri, "pbk=pubkey123") {
		t.Fatalf("expected pbk=pubkey123 in URI, got %s", uri)
	}
	if !strings.Contains(uri, "sid=sid123") {
		t.Fatalf("expected sid=sid123 in URI, got %s", uri)
	}
	if !strings.Contains(uri, "type=xhttp") {
		t.Fatalf("expected type=xhttp in URI, got %s", uri)
	}
}

