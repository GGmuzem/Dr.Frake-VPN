package router

import (
	"strings"
	"time"
	"vpn-backend/internal/config"
	"vpn-backend/internal/handlers"
	"vpn-backend/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	// CORS — configurable origins
	allowedOrigins := cfg.AllowedOrigins
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, allowed := range strings.Split(allowedOrigins, ",") {
				if strings.TrimSpace(allowed) == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		// Vary: Origin нужен когда origin-specific (не "*"), чтобы CDN/Caddy не кешировали неправильно
		if allowedOrigins != "*" {
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Rate limiters
	authLimiter := middleware.NewRateLimiter(10, 1*time.Minute)
	webhookLimiter := middleware.NewRateLimiter(30, 1*time.Minute)

	// Handlers
	authH := handlers.NewAuthHandler(db, cfg)
	userH := handlers.NewUserHandler(db, cfg)
	vpnH := handlers.NewVPNHandler(db)
	payH := handlers.NewPaymentHandler(db, cfg.YooKassaShopID, cfg.YooKassaKey, cfg)
	adminH := handlers.NewAdminHandler(db, cfg)
	happH := handlers.NewHappHandler(db, cfg)
	webConfigH := handlers.NewWebConfigHandler(db, cfg)
	downloadH := handlers.NewDownloadHandler(db)

	auth := middleware.AuthRequired(cfg.JWTSecret)
	admin := middleware.AdminRequired()

	api := r.Group("/api/v1")
	{
		// Public (rate limited)
		authGroup := api.Group("/auth", middleware.RateLimit(authLimiter))
		{
			authGroup.POST("/register", authH.Register)
			authGroup.POST("/verify", authH.VerifyEmail)
			authGroup.POST("/login", authH.Login)
			authGroup.POST("/refresh", authH.Refresh)
			authGroup.POST("/forgot-password", authH.ForgotPassword)
			authGroup.POST("/reset-password", authH.ResetPassword)
			authGroup.POST("/tv/start", authH.TVStart)
			authGroup.POST("/tv/approve", authH.TVApprove)
			authGroup.POST("/tv/token", authH.TVToken)
		}

		// Client Updates (Public)
		api.GET("/client/latest-version", handlers.GetLatestClientVersion(cfg))
		api.GET("/web/config", webConfigH.Get)
		api.GET("/happ/sub/:token", happH.Subscription)

		// TV approve confirmation page, also exposed under /tv at the
		// root for the original device-flow URL. Mounted under the API
		// group too so it is always reachable through reverse proxies
		// that only forward /api/v1/* to the backend.
		api.GET("/tv", authH.TVApprovePage)

		// Webhook — rate limited. The handler verifies the payment by fetching
		// the payment status from YooKassa before activating a subscription.
		api.POST("/payments/webhook",
			middleware.RateLimit(webhookLimiter),
			payH.Webhook,
		)

		// Authenticated
		me := api.Group("/me", auth)
		{
			me.GET("", userH.GetMe)
			me.GET("/subscription", userH.GetSubscription)
			me.GET("/servers", userH.GetServers)
			me.PATCH("/subscription/auto-renew", userH.SetAutoRenew)
			me.PATCH("/subscription/ad-block", userH.SetVIPAdBlock)
			me.DELETE("/card", userH.DeleteCard)
			me.GET("/routing-profiles", userH.GetRoutingProfiles)
			me.POST("/routing-profiles", userH.CreateRoutingProfile)
			me.POST("/routing-profiles/system/:code/copy", userH.CopySystemRoutingProfile)
			me.PUT("/routing-profiles/:id", userH.UpdateRoutingProfile)
			me.DELETE("/routing-profiles/:id", userH.DeleteRoutingProfile)
			me.POST("/support/bug-report", userH.SubmitBugReport)
			me.GET("/config", vpnH.GetConfig)
			me.POST("/config/revoke", vpnH.RevokeConfig)
			me.POST("/happ-link", happH.CreateLink)
			me.POST("/tv/approve", authH.TVApproveAuthenticated)
		}

		payments := api.Group("/payments", auth)
		{
			payments.POST("/preview", payH.PreviewPayment)
			payments.POST("/create", payH.CreatePayment)
		}

		// Admin only
		adminGrp := api.Group("/admin", auth, admin)
		{
			adminGrp.GET("/users", adminH.GetUsers)
			adminGrp.POST("/users/:id/upgrade", adminH.UpgradeUser)
			adminGrp.POST("/users/:id/subscription/revoke", adminH.RevokeUserSubscription)
			adminGrp.POST("/users/:id/revoke", adminH.RevokeUserKeys)
			adminGrp.POST("/users/:id/set-role", adminH.SetUserRole)
			adminGrp.DELETE("/users/:id", adminH.DeleteUser)
			adminGrp.GET("/servers", adminH.GetServers)
			adminGrp.POST("/servers", adminH.AddServer)
			adminGrp.POST("/servers/pihole-sync", adminH.PiHoleSync)
			adminGrp.PUT("/servers/:id", adminH.UpdateServer)
			adminGrp.POST("/servers/:id/toggle", adminH.ToggleServer)
			adminGrp.POST("/servers/:id/agent/bootstrap", adminH.AgentBootstrap)
			adminGrp.POST("/servers/:id/agent/snapshot", adminH.AgentSnapshot)
			adminGrp.POST("/servers/:id/agent/update", adminH.AgentUpdate)
			adminGrp.POST("/servers/:id/agent/rollback", adminH.AgentRollback)
			adminGrp.GET("/servers/:id/agent/status", adminH.AgentStatus)
			adminGrp.DELETE("/servers/:id", adminH.DeleteServer)
			adminGrp.GET("/payments", adminH.GetPayments)
			adminGrp.POST("/payments/:id/approve", adminH.ApprovePayment)
			adminGrp.GET("/promo-codes", adminH.GetPromoCodes)
			adminGrp.POST("/promo-codes", adminH.CreatePromoCode)
			adminGrp.PUT("/promo-codes/:id", adminH.UpdatePromoCode)
			adminGrp.DELETE("/promo-codes/:id", adminH.DeletePromoCode)
			adminGrp.GET("/downloads", adminH.GetDownloads)
			adminGrp.POST("/downloads/:platform", adminH.UploadDownload)
			adminGrp.GET("/stats", adminH.GetStats)
			adminGrp.GET("/export/:entity", adminH.ExportCSV)
			adminGrp.POST("/backup/send", adminH.TriggerBackup)
		}
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Веб-панель администратора
	r.GET("/download/:platform", downloadH.Download)
	r.Static("/admin", "./admin")
	r.GET("/tv", authH.TVApprovePage)
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/admin/")
	})

	return r
}
