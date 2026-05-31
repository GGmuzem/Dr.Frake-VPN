package handlers

import (
	"net/http"
	"strings"
	"vpn-backend/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type importServerConfigsRequest struct {
	AWGConfig      string `json:"awg_config"`
	XrayConfigJSON string `json:"xray_config_json"`
	XrayPublicKey  string `json:"xray_public_key"`
	XrayShortID    string `json:"xray_short_id"`
	XrayClientID   string `json:"xray_client_id"`
	XrayMLDSA65    string `json:"xray_mldsa65_verify"`
}

func (h *AdminHandler) ImportServerConfigs(c *gin.Context) {
	var req importServerConfigsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hasAWG := strings.TrimSpace(req.AWGConfig) != ""
	hasXray := strings.TrimSpace(req.XrayConfigJSON) != ""
	if !hasAWG && !hasXray {
		c.JSON(http.StatusBadRequest, gin.H{"error": "awg_config or xray_config_json is required"})
		return
	}

	var server models.VPNServer
	if err := h.db.First(&server, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}

	var template models.VLESSServerTemplate
	if err := h.db.Where("server_id = ?", server.ID).FirstOrInit(&template, models.VLESSServerTemplate{ServerID: server.ID}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if hasAWG {
		applyAWGSnapshot(&server, req.AWGConfig)
	}
	if hasXray {
		files := map[string][]byte{
			"xray/server.json":             []byte(req.XrayConfigJSON),
			"xray/xray_public.key":         []byte(strings.TrimSpace(req.XrayPublicKey)),
			"xray/xray_short_id.key":       []byte(strings.TrimSpace(req.XrayShortID)),
			"xray/xray_uuid.key":           []byte(strings.TrimSpace(req.XrayClientID)),
			"xray/xray_mldsa65_verify.key": []byte(strings.TrimSpace(req.XrayMLDSA65)),
		}
		if err := applyVLESSSnapshot(&server, &template, files); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid xray_config_json: " + err.Error()})
			return
		}
		if !hasUsableVLESSTemplate(&template) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "xray config must include vless reality serverName, public key and short id"})
			return
		}
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&server).Error; err != nil {
			return err
		}
		if hasXray {
			if err := tx.Save(&template).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	imported := []string{}
	if hasAWG {
		imported = append(imported, "awg")
	}
	if hasXray {
		imported = append(imported, "xray")
	}
	auditServerAction(h.db, c, "server.config_import", server.ID, "ok", strings.Join(imported, ","))
	c.JSON(http.StatusOK, gin.H{
		"message":  "configs imported",
		"imported": imported,
		"server":   adminServerHealthResponse(server),
	})
}

func serverConfigSummary(server models.VPNServer, template *models.VLESSServerTemplate) gin.H {
	result := gin.H{
		"awg": gin.H{
			"listen_port": server.AWGPort,
			"interface":   server.AWGInterface,
			"container":   server.AWGContainer,
			"jc":          server.Jc,
			"jmin":        server.Jmin,
			"jmax":        server.Jmax,
			"s1":          server.S1,
			"s2":          server.S2,
			"s3":          server.S3,
			"s4":          server.S4,
			"h1":          server.H1,
			"h2":          server.H2,
			"h3":          server.H3,
			"h4":          server.H4,
		},
	}
	if template != nil && template.ID != 0 {
		result["xray"] = vlessTemplateResponse(*template)
	}
	return result
}

func vlessTemplateResponse(template models.VLESSServerTemplate) gin.H {
	return gin.H{
		"address":                 template.Address,
		"port":                    template.Port,
		"server_name":             template.ServerName,
		"public_key":              template.PublicKey,
		"short_id":                template.ShortID,
		"short_ids_json":          template.ShortIDsJSON,
		"fingerprint":             template.Fingerprint,
		"flow":                    template.Flow,
		"network":                 template.Network,
		"security":                template.Security,
		"spider_x":                template.SpiderX,
		"mldsa65_verify":          template.MLDSA65Verify,
		"grpc_service_name":       template.GrpcServiceName,
		"grpc_authority":          template.GrpcAuthority,
		"grpc_multi_mode":         template.GrpcMultiMode,
		"hysteria_enabled":        template.HysteriaEnabled,
		"hysteria_port":           template.HysteriaPort,
		"hysteria_password":       template.HysteriaPassword,
		"hysteria_sni":            template.HysteriaSNI,
		"hysteria_insecure":       template.HysteriaInsecure,
		"hysteria_obfs_password":  template.HysteriaObfsPassword,
		"hysteria_masquerade_url": template.HysteriaMasqueradeURL,
		"container_name":          template.ContainerName,
	}
}
