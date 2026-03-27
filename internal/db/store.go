package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore creates or opens the SQLite database at ~/.god-hunter/sessions.db
func NewStore() (*Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbDir := filepath.Join(homeDir, ".god-hunter")
	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &Store{db: db}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			scope_file TEXT,
			status TEXT DEFAULT 'active',
			target_domain TEXT,
			program_name TEXT,
			platform TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS chains (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			name TEXT,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			request_count INTEGER DEFAULT 0,
			vulnerabilities_found INTEGER DEFAULT 0,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS requests (
			id TEXT PRIMARY KEY,
			chain_id TEXT NOT NULL,
			sequence_order INTEGER,
			method TEXT,
			url TEXT,
			headers TEXT,
			body TEXT,
			response_status INTEGER,
			response_headers TEXT,
			response_body TEXT,
			timing_ms INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (chain_id) REFERENCES chains(id)
		)`,
		`CREATE TABLE IF NOT EXISTS findings (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			chain_id TEXT,
			vulnerability_type TEXT,
			severity TEXT,
			endpoint TEXT,
			parameter TEXT,
			description TEXT,
			evidence TEXT,
			poc_payload TEXT,
			status TEXT DEFAULT 'unreviewed',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id),
			FOREIGN KEY (chain_id) REFERENCES chains(id)
		)`,
		`CREATE TABLE IF NOT EXISTS wordlists_entries (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			entry_type TEXT,
			value TEXT,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS iam_graph (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			entity TEXT,
			permission TEXT,
			resource TEXT,
			trust_relationship TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chains_session ON chains(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_chain ON requests(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_findings_session ON findings(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_findings_chain ON findings(chain_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wordlists_session ON wordlists_entries(session_id)`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Session operations
func (s *Store) CreateSession(id, targetDomain, programName, platform string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO sessions (id, target_domain, program_name, platform)
	          VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, id, targetDomain, programName, platform)
	return err
}

func (s *Store) GetSession(id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow("SELECT id, created_at, status, target_domain, program_name, platform FROM sessions WHERE id = ?", id)

	var sessionID, createdAt, status, targetDomain, programName, platform string
	if err := row.Scan(&sessionID, &createdAt, &status, &targetDomain, &programName, &platform); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":            sessionID,
		"created_at":    createdAt,
		"status":        status,
		"target_domain": targetDomain,
		"program_name":  programName,
		"platform":      platform,
	}, nil
}

// Chain operations
func (s *Store) CreateChain(chainID, sessionID, name, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO chains (id, session_id, name, description)
	          VALUES (?, ?, ?, ?)`
	_, err := s.db.Exec(query, chainID, sessionID, name, description)
	return err
}

func (s *Store) StoreRequest(reqID, chainID string, sequence int, method, url, headers, body string,
	responseStatus int, responseHeaders, responseBody string, timingMs int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO requests 
	          (id, chain_id, sequence_order, method, url, headers, body, 
	           response_status, response_headers, response_body, timing_ms)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, reqID, chainID, sequence, method, url, headers, body,
		responseStatus, responseHeaders, responseBody, timingMs)
	return err
}

func (s *Store) GetChainRequests(chainID string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, sequence_order, method, url, headers, body, 
		       response_status, response_headers, response_body, timing_ms
		FROM requests WHERE chain_id = ? ORDER BY sequence_order`,
		chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []map[string]interface{}
	for rows.Next() {
		var id, method, url, headers, body, respHeaders, respBody string
		var seq, respStatus, timing int

		if err := rows.Scan(&id, &seq, &method, &url, &headers, &body,
			&respStatus, &respHeaders, &respBody, &timing); err != nil {
			return nil, err
		}

		requests = append(requests, map[string]interface{}{
			"id":               id,
			"sequence_order":   seq,
			"method":           method,
			"url":              url,
			"headers":          headers,
			"body":             body,
			"response_status":  respStatus,
			"response_headers": respHeaders,
			"response_body":    respBody,
			"timing_ms":        timing,
		})
	}

	return requests, rows.Err()
}

// Finding operations
func (s *Store) StoreFinding(findingID, sessionID, chainID, vulnType, severity, endpoint, parameter,
	description, evidence, pocPayload string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO findings 
	          (id, session_id, chain_id, vulnerability_type, severity, endpoint, parameter, 
	           description, evidence, poc_payload)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := s.db.Exec(query, findingID, sessionID, chainID, vulnType, severity, endpoint,
		parameter, description, evidence, pocPayload)
	return err
}

func (s *Store) GetFindings(sessionID string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, vulnerability_type, severity, endpoint, parameter, description, 
		       evidence, poc_payload, status, created_at
		FROM findings WHERE session_id = ? ORDER BY created_at DESC`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []map[string]interface{}
	for rows.Next() {
		var id, vulnType, severity, endpoint, parameter, description, evidence, pocPayload, status, createdAt string

		if err := rows.Scan(&id, &vulnType, &severity, &endpoint, &parameter, &description,
			&evidence, &pocPayload, &status, &createdAt); err != nil {
			return nil, err
		}

		findings = append(findings, map[string]interface{}{
			"id":                 id,
			"vulnerability_type": vulnType,
			"severity":           severity,
			"endpoint":           endpoint,
			"parameter":          parameter,
			"description":        description,
			"evidence":           evidence,
			"poc_payload":        pocPayload,
			"status":             status,
			"created_at":         createdAt,
		})
	}

	return findings, rows.Err()
}

// GetChainsForSession retrieves all chains for a given session
func (s *Store) GetChainsForSession(sessionID string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, name, description, created_at, request_count 
		FROM chains WHERE session_id = ? ORDER BY created_at DESC`,
		sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chains []map[string]interface{}
	for rows.Next() {
		var id, name, description, createdAt string
		var requestCount int

		if err := rows.Scan(&id, &name, &description, &createdAt, &requestCount); err != nil {
			return nil, err
		}

		chains = append(chains, map[string]interface{}{
			"id":            id,
			"name":          name,
			"description":   description,
			"created_at":    createdAt,
			"request_count": requestCount,
		})
	}

	return chains, rows.Err()
}

// GetRequestsForChain retrieves all requests in a chain
func (s *Store) GetRequestsForChain(chainID string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT id, method, url, headers, body, response_status, response_headers, 
		       response_body, timing_ms, sequence_order, created_at
		FROM requests WHERE chain_id = ? ORDER BY sequence_order ASC`,
		chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []map[string]interface{}
	for rows.Next() {
		var id, method, url, headers, body, respHeaders, respBody, createdAt string
		var respStatus, timingMs, seqOrder int

		if err := rows.Scan(&id, &method, &url, &headers, &body, &respStatus,
			&respHeaders, &respBody, &timingMs, &seqOrder, &createdAt); err != nil {
			return nil, err
		}

		requests = append(requests, map[string]interface{}{
			"id":               id,
			"method":           method,
			"url":              url,
			"headers":          headers,
			"body":             body,
			"response_status":  respStatus,
			"response_headers": respHeaders,
			"response_body":    respBody,
			"timing_ms":        timingMs,
			"sequence_order":   seqOrder,
			"created_at":       createdAt,
		})
	}

	return requests, rows.Err()
}
