package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var imageDigestPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:/-]+@sha256:[a-fA-F0-9]{64}$`)

type UpdateStatus string

const (
	UpdateStatusIdle       UpdateStatus = "idle"
	UpdateStatusSucceeded  UpdateStatus = "succeeded"
	UpdateStatusFailed     UpdateStatus = "failed"
	UpdateStatusRolledBack UpdateStatus = "rolled_back"
)

type UpdateState struct {
	Status        UpdateStatus `json:"status"`
	ActiveDigest  string       `json:"active_digest"`
	PreviousDigest string       `json:"previous_digest"`
	LastError      string       `json:"last_error"`
	UpdatedAt      time.Time    `json:"updated_at"`
	CanRollback    bool         `json:"can_rollback"`
}

type Updater interface {
	Apply(ctx context.Context, digest string) error
	Rollback(ctx context.Context, digest string) error
}

type UpdateManager struct {
	updater Updater
	state   UpdateState
	mu      sync.Mutex
}

func NewUpdateManager(updater Updater) *UpdateManager {
	return &UpdateManager{
		updater: updater,
		state:   UpdateState{Status: UpdateStatusIdle},
	}
}

func NewUpdateManagerWithState(updater Updater, state UpdateState) *UpdateManager {
	if state.Status == "" {
		state.Status = UpdateStatusIdle
	}
	return &UpdateManager{updater: updater, state: state}
}

func (m *UpdateManager) Status() UpdateState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *UpdateManager) Apply(ctx context.Context, digest string) UpdateState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !imageDigestPattern.MatchString(digest) {
		m.state = failState(m.state, "image_digest must be immutable image@sha256 digest")
		return m.state
	}
	previous := m.state.ActiveDigest
	rollbackTarget := m.state.PreviousDigest
	if err := m.updater.Apply(ctx, digest); err != nil {
		m.state = failState(m.state, err.Error())
		if previous != "" {
			_ = m.updater.Rollback(ctx, previous)
			m.state.ActiveDigest = previous
			m.state.PreviousDigest = rollbackTarget
			m.state.CanRollback = rollbackTarget != ""
		}
		return m.state
	}
	m.state = UpdateState{
		Status:         UpdateStatusSucceeded,
		ActiveDigest:   digest,
		PreviousDigest: previous,
		UpdatedAt:      time.Now().UTC(),
		CanRollback:    previous != "",
	}
	return m.state
}

func (m *UpdateManager) Rollback(ctx context.Context) UpdateState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state.PreviousDigest == "" {
		m.state = failState(m.state, "no previous digest recorded")
		return m.state
	}
	if err := m.updater.Rollback(ctx, m.state.PreviousDigest); err != nil {
		m.state = failState(m.state, err.Error())
		return m.state
	}
	active := m.state.ActiveDigest
	m.state = UpdateState{
		Status:         UpdateStatusRolledBack,
		ActiveDigest:   m.state.PreviousDigest,
		PreviousDigest: active,
		UpdatedAt:      time.Now().UTC(),
		CanRollback:    active != "",
	}
	return m.state
}

func failState(current UpdateState, message string) UpdateState {
	current.Status = UpdateStatusFailed
	current.LastError = message
	current.UpdatedAt = time.Now().UTC()
	current.CanRollback = current.PreviousDigest != ""
	return current
}

type CommandUpdater struct {
	UpdateCommand   string
	RollbackCommand string
	DockerBin       string
}

func (u CommandUpdater) Apply(ctx context.Context, digest string) error {
	if u.DockerBin == "" {
		u.DockerBin = "docker"
	}
	if err := exec.CommandContext(ctx, u.DockerBin, "pull", digest).Run(); err != nil {
		return err
	}
	if u.UpdateCommand == "" {
		return errors.New("AGENT_UPDATE_COMMAND is required to restart the agent safely")
	}
	return runDigestCommand(ctx, u.UpdateCommand, digest)
}

func (u CommandUpdater) Rollback(ctx context.Context, digest string) error {
	if u.RollbackCommand == "" {
		return errors.New("AGENT_ROLLBACK_COMMAND is required to roll back safely")
	}
	return runDigestCommand(ctx, u.RollbackCommand, digest)
}

func runDigestCommand(ctx context.Context, command, digest string) error {
	cmd := exec.CommandContext(ctx, command)
	cmd.Env = append(os.Environ(), "FBLINK_TARGET_IMAGE_DIGEST="+digest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func LoadUpdateState(path string) UpdateState {
	if path == "" {
		return UpdateState{Status: UpdateStatusIdle}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return UpdateState{Status: UpdateStatusIdle}
	}
	var state UpdateState
	if err := json.Unmarshal(body, &state); err != nil {
		return UpdateState{Status: UpdateStatusIdle}
	}
	return state
}

func SaveUpdateState(path string, state UpdateState) error {
	if path == "" {
		return nil
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0600)
}
