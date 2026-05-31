package handlers

import (
	"strings"

	"vpn-backend/internal/models"
)

func buildHappClientJSON(clientID string, server *models.VPNServer, template *models.VLESSServerTemplate, profiles []models.RoutingProfile, dnsConfig vipDNSConfig) map[string]interface{} {
	name := strings.TrimSpace(server.Region)
	if name == "" {
		name = strings.TrimSpace(server.Name)
	}
	description := "FBLink VPN"
	if flag := countryCodeToEmoji(server.CountryCode); flag != "" {
		description = flag + " FBLink"
	}
	if server.VIPOnly {
		description += " 👑 VIP"
	}
	description = description + " - " + name

	userConfig := map[string]interface{}{
		"encryption": "none",
		"id":         clientID,
		"level":      8,
	}
	if strings.TrimSpace(template.Flow) != "" {
		userConfig["flow"] = template.Flow
	}

	streamSettings := map[string]interface{}{
		"network": template.Network,
		"security": template.Security,
	}
	if template.Security == "reality" {
		streamSettings["realitySettings"] = map[string]interface{}{
			"allowInsecure": false,
			"fingerprint":   template.Fingerprint,
			"publicKey":     template.PublicKey,
			"serverName":    template.ServerName,
			"shortId":       template.ShortID,
			"show":          false,
			"spiderX":       template.SpiderX,
		}
	}
	switch template.Network {
	case "tcp":
		streamSettings["tcpSettings"] = map[string]interface{}{
			"header": map[string]interface{}{
				"type": "none",
			},
		}
	case "xhttp":
		streamSettings["xhttpSettings"] = map[string]interface{}{
			"path": template.XHTTPPath,
			"host": template.XHTTPHost,
			"mode": template.XHTTPMode,
			"extra": map[string]interface{}{
				"padding":  template.XHTTPPadding,
				"postSize": template.XHTTPPostSize,
			},
		}
	case "grpc":
		streamSettings["grpcSettings"] = map[string]interface{}{
			"serviceName": template.GrpcServiceName,
			"authority":   template.GrpcAuthority,
			"multiMode":   template.GrpcMultiMode,
		}
	}

	var proxyOutbound map[string]interface{}
	if template.HysteriaEnabled {
		hysteriaPort := template.HysteriaPort
		if hysteriaPort <= 0 {
			hysteriaPort = 443
		}
		sni := template.HysteriaSNI
		if sni == "" {
			sni = template.ServerName
		}
		proxyOutbound = map[string]interface{}{
			"protocol": "hysteria2",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": template.Address,
						"port":    hysteriaPort,
						"users": []interface{}{
							map[string]interface{}{
								"password": template.HysteriaPassword,
							},
						},
					},
				},
			},
			"streamSettings": map[string]interface{}{
				"network":  "hysteria2",
				"security": "tls",
				"tlsSettings": map[string]interface{}{
					"serverName":    sni,
					"allowInsecure": template.HysteriaInsecure,
					"insecure":      template.HysteriaInsecure,
					"alpn":          []string{"h3"},
				},
			},
			"tag": "proxy",
		}
	} else {
		proxyOutbound = map[string]interface{}{
			"mux": map[string]interface{}{
				"concurrency":     -1,
				"enabled":         false,
				"xudpConcurrency": 8,
				"xudpProxyUDP443": "reject",
			},
			"protocol": "vless",
			"settings": map[string]interface{}{
				"vnext": []interface{}{
					map[string]interface{}{
						"address": template.Address,
						"port":    template.Port,
						"users": []interface{}{
							userConfig,
						},
					},
				},
			},
			"streamSettings": streamSettings,
			"tag": "proxy",
		}
	}

	return map[string]interface{}{
		"remarks": description,
		"meta": map[string]interface{}{
			"serverDescription": description,
		},
		"dns": map[string]interface{}{
			"hosts": map[string]interface{}{
				"cloudflare-dns.com": "1.1.1.1",
				"dns.google":         "8.8.8.8",
			},
			"queryStrategy": "UseIP",
			"servers": []interface{}{
				"https://cloudflare-dns.com/dns-query",
				map[string]interface{}{
					"address": "https://cloudflare-dns.com/dns-query",
					"domains": []string{},
				},
				map[string]interface{}{
					"address": "https://dns.google/dns-query",
					"domains": []string{
						"full:.ru",
						"full:.xn--p1ai",
					},
				},
			},
		},
		"inbounds": []interface{}{
			map[string]interface{}{
				"listen":   "127.0.0.1",
				"port":     10808,
				"protocol": "socks",
				"settings": map[string]interface{}{
					"auth":      "noauth",
					"udp":       true,
					"userLevel": 8,
				},
				"sniffing": map[string]interface{}{
					"destOverride": []string{"http", "tls", "quic"},
					"enabled":      true,
				},
				"tag": "socks",
			},
			map[string]interface{}{
				"listen":   "127.0.0.1",
				"port":     10809,
				"protocol": "http",
				"settings": map[string]interface{}{
					"userLevel": 8,
				},
				"sniffing": map[string]interface{}{
					"destOverride": []string{"http", "tls", "quic"},
					"enabled":      true,
				},
				"tag": "http",
			},
			map[string]interface{}{
				"listen":   "127.0.0.1",
				"port":     11111,
				"protocol": "dokodemo-door",
				"settings": map[string]interface{}{
					"address": "127.0.0.1",
				},
				"tag": "metrics_in",
			},
		},
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"metrics": map[string]interface{}{
			"tag": "metrics_out",
		},
		"outbounds": []interface{}{
			proxyOutbound,
			map[string]interface{}{
				"protocol": "freedom",
				"settings": map[string]interface{}{
					"domainStrategy": "UseIP",
				},
				"tag": "direct",
			},
			map[string]interface{}{
				"protocol": "blackhole",
				"settings": map[string]interface{}{
					"response": map[string]interface{}{
						"type": "http",
					},
				},
				"tag": "block",
			},
		},
		"policy": map[string]interface{}{
			"levels": map[string]interface{}{
				"0": map[string]interface{}{
					"statsUserDownlink": true,
					"statsUserUplink":   true,
				},
				"8": map[string]interface{}{
					"connIdle":     300,
					"downlinkOnly": 1,
					"handshake":    4,
					"uplinkOnly":   1,
				},
			},
			"system": map[string]interface{}{
				"statsInboundDownlink":  true,
				"statsInboundUplink":    true,
				"statsOutboundDownlink": true,
				"statsOutboundUplink":   true,
			},
		},
		"routing": map[string]interface{}{
			"domainStrategy": "IPIfNonMatch",
			"rules": []interface{}{
				map[string]interface{}{
					"network":     "udp",
					"port":        443,
					"outboundTag": "block",
				},
				map[string]interface{}{
					"ip":          []string{"1.1.1.1"},
					"outboundTag": "proxy",
					"port":        443,
				},
				map[string]interface{}{
					"ip":          []string{"8.8.8.8"},
					"outboundTag": "direct",
					"port":        443,
				},
				map[string]interface{}{
					"inboundTag":  []string{"metrics_in"},
					"outboundTag": "metrics_out",
				},
				map[string]interface{}{
					"domain": []string{
						"full:.ru",
						"full:.xn--p1ai",
					},
					"outboundTag": "direct",
				},
				map[string]interface{}{
					"ip": []string{
						"10.0.0.0/8",
						"172.16.0.0/12",
						"192.168.0.0/16",
						"169.254.0.0/16",
						"224.0.0.0/4",
						"255.255.255.255",
					},
					"outboundTag": "direct",
				},
			},
		},
		"stats": map[string]interface{}{},
	}
}

func (h *HappHandler) happJSONConfigs(userID uint, sub models.Subscription) ([]map[string]interface{}, error) {
	isVIP := isVIPSubscription(sub)
	var servers []models.VPNServer
	query := h.db.Where("active = ?", true)
	if !isVIP {
		query = query.Where("vip_only = ?", false)
	}
	if err := query.Preload("VLESSTemplate").Order("id asc").Find(&servers).Error; err != nil {
		return nil, err
	}

	var profiles []models.RoutingProfile
	if isVIP {
		if err := ensureDefaultRoutingProfiles(h.db, userID); err == nil {
			h.db.Where("user_id = ?", userID).Order("sort_order asc, kind asc, id asc").Find(&profiles)
		}
	}

	configs := make([]map[string]interface{}, 0, len(servers))
	for i := range servers {
		server := &servers[i]
		template := server.VLESSTemplate
		if template != nil {
			xrayTemplateDefaults(template, server)
		}
		if !hasUsableVLESSTemplate(template) {
			continue
		}

		clientID := ""
		if template != nil {
			clientID = strings.TrimSpace(template.ClientID)
		}

		if clientID == "" {
			var credentials []models.VLESSCredential
			err := h.db.
				Where("user_id = ? AND server_id = ? AND revoked_at IS NULL", userID, server.ID).
				Limit(1).
				Find(&credentials).Error
			if err != nil {
				continue
			}
			if len(credentials) == 0 {
				continue
			}
			clientID = strings.TrimSpace(credentials[0].ClientID)
		}

		if clientID == "" {
			continue
		}

		dnsConfig := resolveVIPDNSConfig(h.db, server, template, sub)
		config := buildHappClientJSON(clientID, server, template, profiles, dnsConfig)
		
		configs = append(configs, config)
	}
	return configs, nil
}
