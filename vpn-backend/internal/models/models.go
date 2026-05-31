package models

import (
	"time"

	"gorm.io/gorm"
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	gorm.Model
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Role         Role   `gorm:"default:user"`

	Subscription *Subscription
	VPNKeys      []VPNKey
	Payments     []Payment
}

type PlanType string

const (
	PlanFree    PlanType = "free"
	PlanTrial   PlanType = "trial"
	PlanBasic   PlanType = "basic"
	PlanBasic3M PlanType = "basic_3m"
	PlanVIP     PlanType = "vip"
	PlanVIP3M   PlanType = "vip_3m"
)

type SubscriptionStatus string

const (
	SubActive    SubscriptionStatus = "active"
	SubExpired   SubscriptionStatus = "expired"
	SubCancelled SubscriptionStatus = "cancelled"
)

type Subscription struct {
	gorm.Model
	UserID            uint               `gorm:"uniqueIndex;not null"`
	Plan              PlanType           `gorm:"default:free"`
	Status            SubscriptionStatus `gorm:"default:active"`
	ExpiresAt         time.Time
	AutoRenew         bool   `gorm:"default:false"` // true only when payment method is saved after a successful charge
	PaymentMethodID   string `gorm:"default:''"`    // YooKassa payment_method_id для автосписания
	VIPAdBlockEnabled bool   `gorm:"column:vip_ad_block_enabled;default:false"`
}

type TVLoginStatus string

const (
	TVLoginPending  TVLoginStatus = "pending"
	TVLoginApproved TVLoginStatus = "approved"
	TVLoginConsumed TVLoginStatus = "consumed"
	TVLoginExpired  TVLoginStatus = "expired"
)

type TVLogin struct {
	gorm.Model
	DeviceCodeHash string `gorm:"uniqueIndex;not null"`
	UserCodeHash   string `gorm:"index;not null"`
	UserID         *uint
	Status         TVLoginStatus `gorm:"default:pending;not null"`
	ExpiresAt      time.Time     `gorm:"not null"`
	LastPolledAt   *time.Time
	ApprovedAt     *time.Time
	ConsumedAt     *time.Time

	User *User
}

type AppDownload struct {
	gorm.Model
	Platform     string `gorm:"uniqueIndex;not null"`
	OriginalName string `gorm:"not null"`
	FileName     string `gorm:"not null"`
	Path         string `gorm:"not null"`
	Size         int64  `gorm:"not null"`
}

type VPNServer struct {
	gorm.Model
	Name        string `gorm:"not null"`
	Host        string `gorm:"not null"`
	PublicKey   string `gorm:"not null"`
	MaxPeers    int    `gorm:"default:100"`
	Region      string
	CountryCode string `gorm:"default:''"` // ISO 3166-1 alpha-2, e.g. "RU", "US", "DE"
	Active      bool   `gorm:"default:true"`
	VIPOnly     bool   `gorm:"column:vip_only;default:false"`

	// FBLinkWG 2 параметры (обфускация)
	AWGPort  int    `gorm:"default:51820"`
	Endpoint string // host:port для клиентов (может отличаться от Host)
	MTU      string `gorm:"default:1280"`
	Jc       string `gorm:"default:4"`
	Jmin     string `gorm:"default:40"`
	Jmax     string `gorm:"default:70"`
	S1       string `gorm:"default:0"`
	S2       string `gorm:"default:0"`
	S3       string `gorm:"default:0"`          // AWG2: cookie reply
	S4       string `gorm:"default:0"`          // AWG2: transport
	H1       string `gorm:"not null;default:0"` // range: min-max или фиксированное
	H2       string `gorm:"not null;default:0"`
	H3       string `gorm:"not null;default:0"`
	H4       string `gorm:"not null;default:0"`
	I1       string
	I2       string
	I3       string
	I4       string
	I5       string

	// SSH доступ для управления peers
	SSHHost      string `gorm:"default:''"` // IP для SSH (если отличается от Host)
	SSHPort      int    `gorm:"default:22"`
	SSHUser      string `gorm:"default:'root'"`
	SSHPassword  string // пароль SSH (ili private key)
	AWGContainer string `gorm:"default:'amnezia-awg2'"`
	AWGInterface string `gorm:"default:'awg0'"`

	// Node agent management over the VLESS/Reality management path.
	AgentURL                 string `gorm:"default:''"`
	AgentNodeID              string `gorm:"default:''"`
	AgentLastSnapshotHash    string `gorm:"default:''"`
	AgentLastSnapshotAt      *time.Time
	AgentLastSnapshotStatus  string `gorm:"default:''"`
	AgentLastVersion         string `gorm:"default:''"`
	AgentLastCommit          string `gorm:"default:''"`
	AgentActiveDigest        string `gorm:"default:''"`
	AgentPreviousDigest      string `gorm:"default:''"`
	AgentLastUpdateStatus    string `gorm:"default:''"`
	AgentLastUpdateError     string `gorm:"default:''"`
	AgentBootstrapStatus     string `gorm:"default:''"`
	AgentBootstrapError      string `gorm:"default:''"`
	AgentBootstrapAt         *time.Time
	AgentLastHeartbeatAt     *time.Time
	AgentDockerAvailable     bool   `gorm:"default:false"`
	AgentUptimeSeconds       int64  `gorm:"default:0"`
	AgentLastHealthStatus    string `gorm:"default:''"`
	AgentManagementPort      int    `gorm:"default:0"`
	AgentLocalPort           int    `gorm:"default:0"`
	AgentManagementUUID      string `gorm:"default:''"`
	AgentManagementShortID   string `gorm:"default:''"`
	AgentManagementPublicKey string `gorm:"default:''"`
	AgentPushPublicKey       string `gorm:"default:''"`

	// Pi-hole AdBlock
	PiHoleMode          string `gorm:"default:'auto'"` // auto | host | docker | disabled
	PiHoleContainerName string `gorm:"default:''"`     // override docker container name
	PiHoleGroupName     string `gorm:"default:'VIP'"`  // gravity.db group name
	PiHoleEnabled       bool   `gorm:"default:true"`
	PiHoleDNSIP         string `gorm:"default:''"` // кеш IP после pihole-sync (не ходим по SSH при каждом запросе)
	PiHoleLastSyncAt    *time.Time
	PiHoleLastSyncError string `gorm:"default:''"`
	PiHoleLastMode      string `gorm:"default:''"`
	PiHoleLastClientIP  string `gorm:"default:''"`

	VPNKeys       []VPNKey             `gorm:"foreignKey:ServerID"`
	VLESSTemplate *VLESSServerTemplate `gorm:"foreignKey:ServerID"`
	VLESSClients  []VLESSCredential    `gorm:"foreignKey:ServerID"`
}

type VPNKey struct {
	gorm.Model
	UserID       uint   `gorm:"not null"`
	ServerID     uint   `gorm:"not null"`
	PublicKey    string `gorm:"not null"`
	PresharedKey string // Уникальный PSK для каждого клиента (безопасность)
	ConfigText   string `gorm:"not null"`
	IssuedAt     time.Time
	RevokedAt    *time.Time

	User   User
	Server VPNServer `gorm:"foreignKey:ServerID"`
}

type VLESSServerTemplate struct {
	gorm.Model
	ServerID              uint   `gorm:"uniqueIndex;not null"`
	ClientID              string `gorm:"default:''"`
	Address               string `gorm:"not null"`
	Port                  int    `gorm:"default:443"`
	ServerName            string `gorm:"not null"`
	PublicKey             string `gorm:"not null"`
	ShortID               string `gorm:"not null"`
	ShortIDsJSON          string `gorm:"default:''"` // JSON-кодированный массив всех shortIds сервера
	Fingerprint           string `gorm:"default:'chrome'"`
	Flow                  string `gorm:"default:'xtls-rprx-vision'"`
	Network               string `gorm:"default:'tcp'"`
	Security              string `gorm:"default:'reality'"`
	SpiderX               string `gorm:"default:'/'"`
	MLDSA65Verify         string `gorm:"default:''"` // post-quantum Reality public verify-key для клиента
	GrpcServiceName       string `gorm:"default:'api.v1.VideoDownload'"`
	GrpcAuthority         string `gorm:"default:''"`
	GrpcMultiMode         bool   `gorm:"default:true"`
	HysteriaEnabled       bool   `gorm:"default:false"`
	HysteriaPort          int    `gorm:"default:443"`
	HysteriaPassword      string `gorm:"default:''"`
	HysteriaSNI           string `gorm:"default:''"`
	HysteriaInsecure      bool   `gorm:"default:true"`
	HysteriaObfsPassword  string `gorm:"default:''"`
	HysteriaMasqueradeURL string `gorm:"default:'https://www.microsoft.com'"`
	ContainerName         string `gorm:"default:'amnezia-xray'"`
	// XHTTP transport fields (xray network="xhttp")
	XHTTPPath     string `gorm:"default:''"`
	XHTTPHost     string `gorm:"default:''"`
	XHTTPMode     string `gorm:"default:''"`
	XHTTPPadding  string `gorm:"default:''"`
	XHTTPPostSize int    `gorm:"default:0"`
}

type VLESSCredential struct {
	gorm.Model
	UserID    uint   `gorm:"not null;uniqueIndex:idx_vless_user_server"`
	ServerID  uint   `gorm:"not null;uniqueIndex:idx_vless_user_server"`
	ClientID  string `gorm:"not null"`
	RevokedAt *time.Time

	User   User
	Server VPNServer `gorm:"foreignKey:ServerID"`
}

type HappSubscriptionToken struct {
	gorm.Model
	UserID     uint   `gorm:"not null;index"`
	TokenHash  string `gorm:"uniqueIndex;not null"`
	Label      string `gorm:"default:'iOS Happ'"`
	RevokedAt  *time.Time
	LastUsedAt *time.Time

	User User
}

type RoutingProfileKind string

const (
	RoutingProfileSystem RoutingProfileKind = "system"
	RoutingProfileCustom RoutingProfileKind = "custom"
)

type RoutingProfileAction string

const (
	RoutingProfileDirect RoutingProfileAction = "direct"
	RoutingProfileProxy  RoutingProfileAction = "proxy"
)

type RoutingProfile struct {
	gorm.Model
	UserID             uint                 `gorm:"not null;index"`
	Name               string               `gorm:"not null"`
	Code               string               `gorm:"default:'';index"`
	TemplateCode       string               `gorm:"default:'';index"`
	Kind               RoutingProfileKind   `gorm:"default:'custom'"`
	Action             RoutingProfileAction `gorm:"default:'direct'"`
	Enabled            bool                 `gorm:"default:false"`
	Description        string               `gorm:"default:''"`
	Icon               string               `gorm:"default:''"`
	SortOrder          int                  `gorm:"default:0"`
	DomainsJSON        string               `gorm:"column:domains_json;default:'[]'"`
	DomainSuffixesJSON string               `gorm:"column:domain_suffixes_json;default:'[]'"`
	CIDRsJSON          string               `gorm:"column:cidrs_json;default:'[]'"`

	User User
}

type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentSucceeded PaymentStatus = "succeeded"
	PaymentCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	gorm.Model
	UserID         uint          `gorm:"not null"`
	YooKassaID     string        `gorm:"uniqueIndex"`
	Amount         float64       `gorm:"not null"`
	OriginalAmount float64       `gorm:"not null;default:0"`
	DiscountAmount float64       `gorm:"not null;default:0"`
	Currency       string        `gorm:"default:RUB"`
	Status         PaymentStatus `gorm:"default:pending"`
	Plan           PlanType      `gorm:"not null"`
	ConfirmURL     string        // URL для оплаты (от ЮKassa)
	ConfirmedAt    *time.Time
	PromoCodeID    *uint

	User      User
	PromoCode *PromoCode
}

type PromoCode struct {
	gorm.Model
	Code            string `gorm:"uniqueIndex;not null"`
	Description     string `gorm:"default:''"`
	DiscountPercent int    `gorm:"not null"`
	MaxUses         int    `gorm:"default:0"`
	UsedCount       int    `gorm:"default:0"`
	Active          bool   `gorm:"default:true"`
	ApplicablePlans string `gorm:"default:'all'"`
	OncePerUser     bool   `gorm:"default:true"`
	ExpiresAt       *time.Time

	Payments []Payment
}

type VerificationCode struct {
	gorm.Model
	Email     string    `gorm:"not null;index"`
	Code      string    `gorm:"not null"`
	Purpose   string    `gorm:"not null"` // "verify" | "reset"
	ExpiresAt time.Time `gorm:"not null"`
	Used      bool      `gorm:"default:false"`
}

type AdminNotificationSeverity string

const (
	AdminNotificationSeverityInfo     AdminNotificationSeverity = "info"
	AdminNotificationSeverityWarning  AdminNotificationSeverity = "warning"
	AdminNotificationSeverityCritical AdminNotificationSeverity = "critical"
)

type AdminNotificationStatus string

const (
	AdminNotificationStatusOpen     AdminNotificationStatus = "open"
	AdminNotificationStatusResolved AdminNotificationStatus = "resolved"
)

type AdminNotification struct {
	gorm.Model
	Fingerprint      string                    `gorm:"uniqueIndex;not null"`
	Severity         AdminNotificationSeverity `gorm:"default:info;not null"`
	Status           AdminNotificationStatus   `gorm:"default:open;not null;index"`
	Title            string                    `gorm:"not null"`
	Message          string                    `gorm:"not null"`
	ServerID         *uint                     `gorm:"index"`
	Server           *VPNServer                `gorm:"foreignKey:ServerID"`
	MetadataJSON     string                    `gorm:"column:metadata_json;default:'{}'"`
	LastSeenAt       time.Time                 `gorm:"not null"`
	AcknowledgedAt   *time.Time
	AcknowledgedByID *uint
	MutedUntil       *time.Time
	ResolvedAt       *time.Time
}

type AdminAuditLog struct {
	gorm.Model
	ActorUserID  *uint  `gorm:"index"`
	Action       string `gorm:"not null;index"`
	Entity       string `gorm:"not null;index"`
	EntityID     string `gorm:"default:'';index"`
	Result       string `gorm:"not null;default:'ok'"`
	Message      string `gorm:"default:''"`
	IP           string `gorm:"default:''"`
	MetadataJSON string `gorm:"column:metadata_json;default:'{}'"`
}
