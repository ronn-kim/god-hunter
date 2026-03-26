package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/god-hunter/god-hunter/internal/db"
)

type Session struct {
	ID           string
	TargetDomain string
	ProgramName  string
	Platform     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Status       string
	store        *db.Store
}

// NewSession creates a new session or retrieves an existing one
func NewSession(sessionID, targetDomain, programName, platform string, store *db.Store) (*Session, error) {
	// Check if session exists
	existing, err := store.GetSession(sessionID)
	if err == nil && existing != nil {
		return &Session{
			ID:           sessionID,
			TargetDomain: existing["target_domain"].(string),
			ProgramName:  existing["program_name"].(string),
			Platform:     existing["platform"].(string),
			Status:       existing["status"].(string),
			store:        store,
		}, nil
	}

	// Create new session
	if err := store.CreateSession(sessionID, targetDomain, programName, platform); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Session{
		ID:           sessionID,
		TargetDomain: targetDomain,
		ProgramName:  programName,
		Platform:     platform,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       "active",
		store:        store,
	}, nil
}

// GenerateChainID creates a deterministic chain identifier
func (s *Session) GenerateChainID(endpoint string) string {
	hash := sha256.Sum256([]byte(s.ID + endpoint + time.Now().Format(time.RFC3339)))
	return "chain_" + hex.EncodeToString(hash[:8])
}

// GenerateRequestID creates a unique request identifier
func (s *Session) GenerateRequestID() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d%s", time.Now().UnixNano(), s.ID)))
	return "req_" + hex.EncodeToString(hash[:8])
}

// GenerateFindingID creates a unique finding identifier
func (s *Session) GenerateFindingID(vulnType, endpoint string) string {
	hash := sha256.Sum256([]byte(s.ID + vulnType + endpoint + time.Now().Format(time.RFC3339Nano)))
	return "vuln_" + hex.EncodeToString(hash[:8])
}

// GetStore returns the underlying database store
func (s *Session) GetStore() *db.Store {
	return s.store
}
