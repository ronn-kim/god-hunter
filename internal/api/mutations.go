package api

import (
	"context"
	"fmt"

	"github.com/god-hunter/god-hunter/internal/db"
	httpengine "github.com/god-hunter/god-hunter/internal/http"
	"github.com/god-hunter/god-hunter/internal/log"
	"github.com/god-hunter/god-hunter/internal/mutations"
	"github.com/god-hunter/god-hunter/internal/session"
	"github.com/god-hunter/god-hunter/internal/validation"
)

// TestMutations runs mutation testing on a stored chain
func TestMutations(ctx context.Context, args []string, sessionName string, silent bool) error {
	logger := log.NewLogger(silent)

	if sessionName == "" {
		return fmt.Errorf("session name required: use --session flag")
	}

	// Validate session name
	if err := validation.ValidateSessionName(sessionName); err != nil {
		return fmt.Errorf("invalid session name: %w", err)
	}

	logger.Info("initializing mutation testing for session: %s", sessionName)

	// Initialize store
	store, err := db.NewStore()
	if err != nil {
		return fmt.Errorf("failed to initialize store: %w", err)
	}
	defer store.Close()

	// Get session
	sessData, err := store.GetSession(sessionName)
	if err != nil || sessData == nil {
		logger.Error("session not found: %s", sessionName)
		return fmt.Errorf("session not found: %s", sessionName)
	}

	sess := &session.Session{
		ID:           sessionName,
		TargetDomain: sessData["target_domain"].(string),
		ProgramName:  sessData["program_name"].(string),
		Platform:     sessData["platform"].(string),
		Status:       sessData["status"].(string),
	}

	logger.Info("found session: %s (target: %s)", sess.ID, sess.TargetDomain)

	// Get chains for this session
	chains, err := store.GetChainsForSession(sess.ID)
	if err != nil || len(chains) == 0 {
		logger.Error("no chains found for session: %s", sessionName)
		return fmt.Errorf("no chains found for session: %s", sessionName)
	}

	logger.Info("found %d chains to test", len(chains))

	// Initialize HTTP client for mutations
	logger.Info("initializing HTTP client for mutation testing")
	client, err := httpengine.NewClientWithLogger("800-2400", "", logger)
	if err != nil {
		return fmt.Errorf("failed to initialize HTTP client: %w", err)
	}
	defer client.Close()

	// Create mutator
	mutator := mutations.NewMutator()
	executor := mutations.NewExecutor(client, logger)

	totalVulnerabilities := 0

	// Test each chain
	for chainIdx, chainData := range chains {
		chainID := chainData["id"].(string)
		logger.Info("testing chain [%d/%d]: %s", chainIdx+1, len(chains), chainID)

		// Convert to mutations.Chain format
		mutChain, err := convertToMutationChain(store, chainID)
		if err != nil {
			logger.Warn("failed to load chain %s: %v", chainID, err)
			continue
		}

		// Generate all mutations
		logger.Info("generating mutations for chain: %s", chainID)
		mutationVariants, err := mutator.MutateAll(mutChain)
		if err != nil {
			logger.Error("failed to generate mutations: %v", err)
			continue
		}

		logger.Info("generated %d mutation variants", len(mutationVariants))

		// Execute comparisons
		results, err := executor.CompareMutations(ctx, mutChain, mutationVariants)
		if err != nil {
			logger.Error("mutation execution failed: %v", err)
			continue
		}

		logger.Info("mutation comparison complete: %d results", len(results))

		// Detect vulnerabilities
		vulns := executor.DetectVulnerabilities()
		logger.Info("chain testing complete: found %d vulnerabilities", len(vulns))

		// Store findings
		for _, vuln := range vulns {
			findingID := sess.GenerateFindingID("MUTATION", chainID)
			store.StoreFinding(
				findingID, sess.ID, chainID, "MUTATION_BASED_VULNERABILITY", vuln.Severity,
				chainID, "mutation_type", vuln.Description,
				fmt.Sprintf("Type: %s; Severity: %s", vuln.MutationType, vuln.Severity),
				vuln.VulnerabilityID,
			)
			totalVulnerabilities++
			logger.Warn("stored finding: %s", findingID)
		}
	}

	logger.Info("mutation testing complete: found %d total vulnerabilities", totalVulnerabilities)

	return nil
}

// convertToMutationChain converts database chain to mutations.Chain
func convertToMutationChain(store *db.Store, chainID string) (*mutations.Chain, error) {
	requests, err := store.GetRequestsForChain(chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch requests: %w", err)
	}

	chain := &mutations.Chain{
		ID:       chainID,
		Requests: make([]mutations.Request, len(requests)),
	}

	for i, req := range requests {
		headers := make(map[string]string)
		if headersStr, ok := req["headers"].(string); ok {
			// Parse JSON headers if needed
			_ = headersStr // TODO: Parse JSON if needed
		}

		chain.Requests[i] = mutations.Request{
			ID:      req["id"].(string),
			Method:  req["method"].(string),
			URL:     req["url"].(string),
			Headers: headers,
			Body:    req["body"].(string),
			Delay:   0, // Default, can be configured
		}
	}

	return chain, nil
}
