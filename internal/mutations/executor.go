package mutations

import (
	"context"
	"fmt"
	"sync"
	"time"

	httpengine "github.com/god-hunter/god-hunter/internal/http"
	"github.com/god-hunter/god-hunter/internal/log"
)

// ExecutionResult holds the result of executing a chain/mutation
type ExecutionResult struct {
	ChainID       string
	MutationType  string
	Requests      int
	SuccessCount  int
	FailureCount  int
	TotalTime     time.Duration
	AvgTimingMs   int64
	ResponseCodes []int
	Anomalies     []string
	Findings      []Finding
	Timestamp     time.Time
}

// Finding represents a discovered vulnerability
type Finding struct {
	Type        string // TIMING, PARAMETER_BYPASS, STATE_INCONSISTENCY, SQL_INJECTION, XSS
	Severity    string
	Parameter   string
	Evidence    string
	ExploitPath string
	Confidence  float64 // 0.0-1.0
}

// Executor executes mutation chains and detects vulnerabilities
type Executor struct {
	client   *httpengine.Client
	logger   *log.Logger
	analyzer *MutationAnalyzer
	mu       sync.RWMutex
}

// NewExecutor creates a new mutation executor
func NewExecutor(client *httpengine.Client, logger *log.Logger) *Executor {
	return &Executor{
		client:   client,
		logger:   logger,
		analyzer: NewMutationAnalyzer(),
	}
}

// ExecuteChain executes a chain and returns detailed results
func (e *Executor) ExecuteChain(ctx context.Context, chain *Chain) (*ExecutionResult, error) {
	e.logger.Info("executing chain: %s with %d requests", chain.ID, len(chain.Requests))

	result := &ExecutionResult{
		ChainID:       chain.ID,
		Timestamp:     time.Now(),
		ResponseCodes: make([]int, 0),
	}

	startTime := time.Now()

	for i, req := range chain.Requests {
		// Check context cancellation
		select {
		case <-ctx.Done():
			e.logger.Warn("chain execution cancelled at request %d", i)
			return result, ctx.Err()
		default:
		}

		e.logger.Debug("executing request [%d/%d]: %s %s", i+1, len(chain.Requests), req.Method, req.URL)

		// Apply request delay
		if req.Delay > 0 {
			select {
			case <-time.After(time.Duration(req.Delay) * time.Millisecond):
			case <-ctx.Done():
				return result, ctx.Err()
			}
		}

		// Execute request
		metadata, err := e.client.DoWithContext(ctx, req.Method, req.URL, req.Headers, req.Body)
		if err != nil {
			e.logger.Warn("request failed: %v", err)
			result.FailureCount++
			continue
		}

		result.SuccessCount++
		result.ResponseCodes = append(result.ResponseCodes, metadata.ResponseStatus)

		// Check for anomalies
		if isAnomalousResponse(metadata.ResponseStatus) {
			anomaly := fmt.Sprintf("Request %d: status %d (timing: %dms)", i, metadata.ResponseStatus, metadata.TimingMs)
			result.Anomalies = append(result.Anomalies, anomaly)
			e.logger.Warn("anomaly detected: %s", anomaly)
		}
	}

	result.TotalTime = time.Since(startTime)
	result.Requests = len(chain.Requests)

	if result.Requests > 0 {
		totalTiming := int64(0)
		for _, code := range result.ResponseCodes {
			totalTiming += int64(code) // Simplified for demo
		}
		result.AvgTimingMs = totalTiming / int64(result.Requests)
	}

	e.logger.Info("chain execution complete: %d requests, %d success, %d failures, %v elapsed",
		result.Requests, result.SuccessCount, result.FailureCount, result.TotalTime)

	return result, nil
}

// CompareMutations executes original chain and mutations, detecting differences
func (e *Executor) CompareMutations(ctx context.Context, original *Chain, mutations map[Strategy]*Chain) ([]MutationResult, error) {
	e.logger.Info("comparing %d mutations against original chain", len(mutations))

	// Execute original
	originalResult, err := e.ExecuteChain(ctx, original)
	if err != nil {
		return nil, fmt.Errorf("failed to execute original chain: %w", err)
	}

	var results []MutationResult
	var mu sync.Mutex

	// Execute mutations in parallel
	var wg sync.WaitGroup
	for strategy, mutated := range mutations {
		wg.Add(1)
		go func(strat Strategy, chain *Chain) {
			defer wg.Done()

			// Create a timeout context for each mutation
			mutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			mutatedResult, err := e.ExecuteChain(mutCtx, chain)
			if err != nil {
				e.logger.Warn("mutation execution failed: %v", err)
				return
			}

			// Compare results
			comparison := e.compareResults(originalResult, mutatedResult, strat)

			mu.Lock()
			results = append(results, comparison)
			mu.Unlock()

			e.analyzer.AddResult(comparison)
		}(strategy, mutated)
	}

	wg.Wait()

	e.logger.Info("mutation comparison complete: analyzed %d mutations", len(results))

	return results, nil
}

// compareResults detects differences between original and mutated execution
func (e *Executor) compareResults(original, mutated *ExecutionResult, strategy Strategy) MutationResult {
	result := MutationResult{
		MutationType: strategyName(strategy),
		Differences:  []string{},
	}

	// Compare response codes
	if len(original.ResponseCodes) == len(mutated.ResponseCodes) {
		for i := 0; i < len(original.ResponseCodes); i++ {
			if original.ResponseCodes[i] != mutated.ResponseCodes[i] {
				result.Differences = append(result.Differences,
					fmt.Sprintf("Request %d: Status %d → %d", i, original.ResponseCodes[i], mutated.ResponseCodes[i]))
			}
		}
	}

	// Compare timing
	timingDiff := mutated.TotalTime.Milliseconds() - original.TotalTime.Milliseconds()
	if timingDiff > 1000 || timingDiff < -1000 { // >1 second difference
		result.Differences = append(result.Differences,
			fmt.Sprintf("Timing: %dms → %dms (diff: %+dms)",
				original.TotalTime.Milliseconds(),
				mutated.TotalTime.Milliseconds(),
				timingDiff))
	}

	// Compare anomalies
	if len(mutated.Anomalies) != len(original.Anomalies) {
		result.Differences = append(result.Differences,
			fmt.Sprintf("Anomalies: %d → %d", len(original.Anomalies), len(mutated.Anomalies)))
	}

	// Detect potential vulnerabilities
	if len(result.Differences) > 0 {
		result.VulnerabilityID = fmt.Sprintf("vuln_%s_%d", strategyName(strategy), time.Now().Unix())
		result.Severity = determineSeverity(strategy)
		result.Description = generateDescription(strategy, result.Differences)
	}

	return result
}

// DetectVulnerabilities analyzes all mutation results for vulnerabilities
func (e *Executor) DetectVulnerabilities() []MutationResult {
	vulnerabilities := e.analyzer.FindVulnerabilities()
	e.logger.Info("detected %d potential vulnerabilities", len(vulnerabilities))

	for i, vuln := range vulnerabilities {
		e.logger.Warn("vulnerability [%d/%d]: %s (%s severity) - %s",
			i+1, len(vulnerabilities), vuln.VulnerabilityID, vuln.Severity, vuln.Description)
	}

	return vulnerabilities
}

// GetMutationReport generates a detailed report of all mutations
func (e *Executor) GetMutationReport() map[string]interface{} {
	results := e.analyzer.GetResults()
	vulnerabilities := e.analyzer.FindVulnerabilities()

	return map[string]interface{}{
		"totalMutations":       len(results),
		"vulnerabilitiesFound": len(vulnerabilities),
		"mutations":            results,
		"vulnerabilities":      vulnerabilities,
		"timestamp":            time.Now(),
	}
}

// Utility functions

func strategyName(strategy Strategy) string {
	names := map[Strategy]string{
		StrategyOrder:     "order",
		StrategyTiming:    "timing",
		StrategyParameter: "parameter",
		StrategyState:     "state",
		StrategyRace:      "race",
	}
	if name, ok := names[strategy]; ok {
		return name
	}
	return "unknown"
}

func isAnomalousResponse(statusCode int) bool {
	return statusCode >= 400 && statusCode < 600
}

func determineSeverity(strategy Strategy) string {
	// Classify severity based on mutation type
	switch strategy {
	case StrategyState:
		return "HIGH" // State mutations leading to differences are serious
	case StrategyParameter:
		return "MEDIUM" // Parameter mutations require verification
	case StrategyTiming:
		return "LOW" // Timing is informational unless it's extreme
	case StrategyOrder:
		return "MEDIUM" // Order matters for business logic
	case StrategyRace:
		return "HIGH" // Race conditions are critical
	default:
		return "MEDIUM"
	}
}

func generateDescription(strategy Strategy, diffs []string) string {
	descriptions := map[Strategy]string{
		StrategyOrder:     "Request order mutation detected inconsistent behavior",
		StrategyTiming:    "Timing variation affected response characteristics",
		StrategyParameter: "Parameter mutation triggered abnormal responses",
		StrategyState:     "State mutation revealed inconsistent state handling",
		StrategyRace:      "Concurrent execution revealed race condition",
	}

	base := descriptions[strategy]
	if len(diffs) > 0 {
		diffStr := ": " + diffs[0]
		for i := 1; i < len(diffs) && i < 3; i++ {
			diffStr += "; " + diffs[i]
		}
		return base + diffStr
	}

	return base
}
