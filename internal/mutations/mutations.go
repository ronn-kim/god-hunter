package mutations

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// Strategy defines the type of mutation to apply
type Strategy int

const (
	StrategyNone      Strategy = iota
	StrategyOrder              // Randomize request sequence
	StrategyTiming             // Inject timing variations
	StrategyParameter          // Mutate request parameters
	StrategyState              // Track and corrupt state
	StrategyRace               // Send concurrent variations
	StrategyAll                // Combine all strategies
)

// Request represents a single API request for mutation
type Request struct {
	ID      string
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Delay   int // milliseconds to wait before sending
}

// Chain represents a sequence of requests
type Chain struct {
	ID       string
	Requests []Request
}

// MutationResult holds the outcome of a mutation attempt
type MutationResult struct {
	MutationType    string
	Original        *Chain
	Mutated         *Chain
	Differences     []string // Differences from original
	VulnerabilityID string   // If vulnerability detected
	Severity        string   // HIGH, MEDIUM, LOW
	Description     string
	Timestamp       time.Time
}

// Mutator applies mutations to chains
type Mutator struct {
	seed            int64
	vulnerabilities []MutationResult
}

// NewMutator creates a new mutator instance
func NewMutator() *Mutator {
	return &Mutator{
		seed:            time.Now().UnixNano(),
		vulnerabilities: []MutationResult{},
	}
}

// Mutate applies a mutation strategy to a chain
func (m *Mutator) Mutate(chain *Chain, strategy Strategy) (*Chain, error) {
	if chain == nil {
		return nil, fmt.Errorf("chain cannot be nil")
	}

	if len(chain.Requests) == 0 {
		return nil, fmt.Errorf("chain has no requests")
	}

	mutated := &Chain{
		ID:       chain.ID + "_mutated_" + fmt.Sprintf("%d", m.seed%10000),
		Requests: make([]Request, len(chain.Requests)),
	}

	// Deep copy requests
	copy(mutated.Requests, chain.Requests)

	switch strategy {
	case StrategyOrder:
		m.mutateOrder(mutated)
	case StrategyTiming:
		m.mutateTiming(mutated)
	case StrategyParameter:
		m.mutateParameters(mutated)
	case StrategyState:
		m.mutateState(mutated)
	case StrategyRace:
		m.mutateRace(mutated)
	case StrategyAll:
		m.mutateOrder(mutated)
		m.mutateTiming(mutated)
		m.mutateParameters(mutated)
	default:
		return chain, nil
	}

	return mutated, nil
}

// mutateOrder randomizes the request sequence
func (m *Mutator) mutateOrder(chain *Chain) {
	if len(chain.Requests) < 2 {
		return
	}

	// Create index mapping for tracking
	indices := make([]int, len(chain.Requests))
	for i := range indices {
		indices[i] = i
	}

	// Fisher-Yates shuffle
	for i := len(indices) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}

	// Reorder requests
	newRequests := make([]Request, len(chain.Requests))
	for i, idx := range indices {
		newRequests[i] = chain.Requests[idx]
	}

	copy(chain.Requests, newRequests)
}

// mutateTiming injects timing variations
func (m *Mutator) mutateTiming(chain *Chain) {
	// Strategy 1: Remove delays (race condition detection)
	for i := range chain.Requests {
		if rand.Float64() < 0.3 { // 30% chance
			chain.Requests[i].Delay = 0
		}
	}

	// Strategy 2: Add jitter
	for i := range chain.Requests {
		if rand.Float64() < 0.2 { // 20% chance
			jitter := rand.Intn(5000) - 2500 // -2500 to +2500 ms
			chain.Requests[i].Delay = max(0, chain.Requests[i].Delay+jitter)
		}
	}

	// Strategy 3: Exponential backoff corruption
	for i := range chain.Requests {
		if rand.Float64() < 0.15 { // 15% chance
			// Reverse exponential backoff
			chain.Requests[i].Delay = int(math.Pow(2, float64(i)) * 100)
		}
	}
}

// mutateParameters modifies request bodies and URLs
func (m *Mutator) mutateParameters(chain *Chain) {
	payloads := []string{
		"' OR '1'='1",
		"' UNION SELECT NULL--",
		"<img src=x onerror=alert(1)>",
		"../../../etc/passwd",
		"{\"bypass\": true}",
		"admin=true&user=attacker",
		"\";DROP TABLE users;--",
		"${7*7}",
		"{{7*7}}",
		"#{7*7}",
	}

	for i := range chain.Requests {
		if rand.Float64() < 0.4 { // 40% chance per request
			// Mutate body if present
			if len(chain.Requests[i].Body) > 0 {
				selectedPayload := payloads[rand.Intn(len(payloads))]
				chain.Requests[i].Body = injectIntoBody(chain.Requests[i].Body, selectedPayload)
			}

			// Mutate URL if has query params
			if strings.Contains(chain.Requests[i].URL, "?") {
				selectedPayload := payloads[rand.Intn(len(payloads))]
				chain.Requests[i].URL = injectIntoURL(chain.Requests[i].URL, selectedPayload)
			}
		}
	}
}

// mutateState tracks state changes and detects inconsistencies
func (m *Mutator) mutateState(chain *Chain) {
	// Add state tracking headers
	for i := range chain.Requests {
		if chain.Requests[i].Headers == nil {
			chain.Requests[i].Headers = make(map[string]string)
		}

		// Track transaction state
		chain.Requests[i].Headers["X-Transaction-ID"] = fmt.Sprintf("txn_%d", i)

		// Add idempotency key variations
		if rand.Float64() < 0.5 {
			chain.Requests[i].Headers["X-Idempotency-Key"] = fmt.Sprintf("key_%d", i)
		} else {
			// Repeat same key (should be idempotent)
			chain.Requests[i].Headers["X-Idempotency-Key"] = "key_0"
		}
	}
}

// mutateRace sends requests with minimal delays
func (m *Mutator) mutateRace(chain *Chain) {
	for i := range chain.Requests {
		// Set minimal delays for concurrent execution
		chain.Requests[i].Delay = rand.Intn(100) // 0-100ms
	}
}

// MutateAll generates all mutation variants
func (m *Mutator) MutateAll(chain *Chain) (map[Strategy]*Chain, error) {
	strategies := []Strategy{
		StrategyOrder,
		StrategyTiming,
		StrategyParameter,
		StrategyState,
		StrategyRace,
	}

	results := make(map[Strategy]*Chain)

	for _, strategy := range strategies {
		mutated, err := m.Mutate(chain, strategy)
		if err != nil {
			return nil, fmt.Errorf("mutation failed for strategy %d: %w", strategy, err)
		}
		results[strategy] = mutated
	}

	return results, nil
}

// Utility functions

func injectIntoBody(body, payload string) string {
	if !strings.Contains(body, "=") {
		return body + "&injected=" + payload
	}

	// Find last parameter
	parts := strings.Split(body, "&")
	if len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if strings.Contains(lastPart, "=") {
			keyVal := strings.Split(lastPart, "=")
			parts[len(parts)-1] = keyVal[0] + "=" + payload
			return strings.Join(parts, "&")
		}
	}

	return body + "&injected=" + payload
}

func injectIntoURL(urlStr, payload string) string {
	if !strings.Contains(urlStr, "?") {
		return urlStr + "?inject=" + payload
	}

	// Append to query string
	return urlStr + "&inject=" + payload
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// AnalyzeMutationResults compares original chain response with mutations
type MutationAnalyzer struct {
	results []MutationResult
}

// NewMutationAnalyzer creates a new analyzer
func NewMutationAnalyzer() *MutationAnalyzer {
	return &MutationAnalyzer{
		results: []MutationResult{},
	}
}

// AddResult records a mutation result
func (ma *MutationAnalyzer) AddResult(result MutationResult) {
	result.Timestamp = time.Now()
	ma.results = append(ma.results, result)
}

// FindVulnerabilities analyzes results for potential vulnerabilities
func (ma *MutationAnalyzer) FindVulnerabilities() []MutationResult {
	var vulnerabilities []MutationResult

	for _, result := range ma.results {
		// Vulnerability heuristics
		if detectVulnerabilities(result) {
			vulnerabilities = append(vulnerabilities, result)
		}
	}

	// Sort by severity
	sort.Slice(vulnerabilities, func(i, j int) bool {
		severityOrder := map[string]int{"HIGH": 3, "MEDIUM": 2, "LOW": 1}
		return severityOrder[vulnerabilities[i].Severity] > severityOrder[vulnerabilities[j].Severity]
	})

	return vulnerabilities
}

// GetResults returns all mutation results
func (ma *MutationAnalyzer) GetResults() []MutationResult {
	return ma.results
}

// detectVulnerabilities applies heuristics to identify vulnerabilities
func detectVulnerabilities(result MutationResult) bool {
	// If vulnerability already identified
	if result.VulnerabilityID != "" {
		return true
	}

	// Check for significant differences in mutation effects
	return len(result.Differences) > 0
}
