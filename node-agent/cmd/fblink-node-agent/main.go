package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"fblink-node-agent/internal/agent"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	cfg := loadConfig()
	publicKey, err := agent.ParsePublicKey(cfg.VerifyKey)
	if err != nil {
		log.Fatalf("invalid AGENT_VERIFY_PUBLIC_KEY: %v", err)
	}

	runner := agent.CommandRunner{}
	snapshotBuilder := agent.NewSnapshotBuilder(agent.SnapshotConfig{
		NodeID:          cfg.NodeID,
		DockerBin:       cfg.DockerBin,
		XrayContainer:   cfg.XrayContainer,
		AWGContainer:    cfg.AWGContainer,
		AWGInterface:    cfg.AWGInterface,
		PiHoleContainer: cfg.PiHoleContainer,
	}, runner)
	updateManager := agent.NewUpdateManagerWithState(agent.CommandUpdater{
		UpdateCommand:   cfg.UpdateCommand,
		RollbackCommand: cfg.RollbackCommand,
		DockerBin:       cfg.DockerBin,
	}, agent.LoadUpdateState(cfg.StatePath))

	server := agent.NewServer(agent.ServerConfig{
		Version:         version,
		Commit:          commit,
		NodeID:          cfg.NodeID,
		ListenAddr:      cfg.ListenAddr,
		StatePath:       cfg.StatePath,
		DockerBin:       cfg.DockerBin,
		Verifier:        agent.NewVerifier(publicKey, time.Duration(cfg.MaxSkewSeconds)*time.Second),
		SnapshotBuilder: snapshotBuilder,
		UpdateManager:   updateManager,
	})

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("fblink-node-agent %s (%s) listening on %s", version, commit, cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

type config struct {
	ListenAddr      string
	NodeID          string
	VerifyKey       string
	MaxSkewSeconds  int
	DockerBin       string
	XrayContainer   string
	AWGContainer    string
	AWGInterface    string
	PiHoleContainer string
	UpdateCommand   string
	RollbackCommand string
	StatePath       string
}

func loadConfig() config {
	return config{
		ListenAddr:      env("AGENT_ADDR", "127.0.0.1:9090"),
		NodeID:          env("AGENT_NODE_ID", "unknown"),
		VerifyKey:       env("AGENT_VERIFY_PUBLIC_KEY", ""),
		MaxSkewSeconds:  envInt("AGENT_ALLOWED_SKEW_SECONDS", 300),
		DockerBin:       env("AGENT_DOCKER_BIN", "docker"),
		XrayContainer:   env("AGENT_XRAY_CONTAINER", "amnezia-xray"),
		AWGContainer:    env("AGENT_AWG_CONTAINER", "amnezia-awg2"),
		AWGInterface:    env("AGENT_AWG_INTERFACE", "awg0"),
		PiHoleContainer: env("AGENT_PIHOLE_CONTAINER", "pihole"),
		UpdateCommand:   env("AGENT_UPDATE_COMMAND", ""),
		RollbackCommand: env("AGENT_ROLLBACK_COMMAND", ""),
		StatePath:       env("AGENT_STATE_PATH", "/var/lib/fblink-node-agent/update-state.json"),
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
