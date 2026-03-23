package llm

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RouterConfig holds configuration for the LLM router.
type RouterConfig struct {
	BYOCMode bool // If true, ALL traffic goes to Ollama (data sovereignty)

	// Provider configurations
	Claude ProviderConfig
	Gemini ProviderConfig
	OpenAI ProviderConfig
	Ollama ProviderConfig

	// Default provider for unclassified incidents
	DefaultProvider Provider
	DefaultModel    string
}

// Router selects the optimal LLM provider for each incident based on:
// (1) data sovereignty policy, (2) severity + complexity, (3) cost optimization.
type Router struct {
	mu      sync.RWMutex
	config  RouterConfig
	clients map[Provider]LLMClient
	logger  *slog.Logger

	// Circuit breaker state per provider
	failures map[Provider]int
	lastFail map[Provider]time.Time
}

// NewRouter creates a new LLM router with the given provider clients.
func NewRouter(config RouterConfig, logger *slog.Logger) *Router {
	return &Router{
		config:   config,
		clients:  make(map[Provider]LLMClient),
		logger:   logger,
		failures: make(map[Provider]int),
		lastFail: make(map[Provider]time.Time),
	}
}

// RegisterClient registers an LLM client for a provider.
func (r *Router) RegisterClient(provider Provider, client LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[provider] = client
	r.logger.Info("registered LLM provider", "provider", provider, "model", client.Model())
}

// SelectProvider implements the 3-factor routing decision from spec §2.2.
//
// Factor 1: Data sovereignty override → BYOC always routes to Ollama
// Factor 2: Severity-based routing:
//   - P0 CRITICAL  → Claude Opus (best reasoning for novel incidents)
//   - P1 HIGH      → Claude Sonnet (strong RCA, reasonable cost)
//   - P2 MEDIUM    → Gemini Flash (high volume, structured output)
//   - P3 LOW       → Gemini Flash-Lite (cheapest, batch-friendly)
// Factor 3: Type specialization:
//   - APP_CRASH (CrashLoopBackOff) → OpenAI GPT-4o (best code analysis)
func (r *Router) SelectProvider(severity Severity, incidentType IncidentType) ProviderConfig {
	// Rule 1: Data sovereignty override
	if r.config.BYOCMode {
		r.logger.Debug("BYOC mode: routing to Ollama", "severity", severity)
		return r.config.Ollama
	}

	// Rule 2: Type specialization — APP_CRASH goes to OpenAI
	if incidentType == IncidentCodeCrash {
		if r.isProviderHealthy(ProviderOpenAI) {
			r.logger.Debug("APP_CRASH routing to OpenAI GPT-4o")
			return r.config.OpenAI
		}
		// Fall through to severity-based if OpenAI is down
	}

	// Rule 3: Severity-based routing
	switch severity {
	case SeverityP0Emergency:
		// P0: Claude Opus — best reasoning for novel, ambiguous incidents
		cfg := r.config.Claude
		cfg.Model = "claude-opus-4-6"
		if r.isProviderHealthy(ProviderClaude) {
			return cfg
		}
		// Fallback: OpenAI GPT-4o for P0 if Claude is down
		if r.isProviderHealthy(ProviderOpenAI) {
			return r.config.OpenAI
		}

	case SeverityP1Critical:
		// P1: Claude Sonnet — strong RCA at moderate cost
		cfg := r.config.Claude
		cfg.Model = "claude-sonnet-4-6"
		if r.isProviderHealthy(ProviderClaude) {
			return cfg
		}
		// Fallback: Gemini Pro
		if r.isProviderHealthy(ProviderGemini) {
			cfg := r.config.Gemini
			cfg.Model = "gemini-3-pro"
			return cfg
		}

	case SeverityP2High:
		// P2: Gemini Flash — high volume, structured JSON, cost-effective
		cfg := r.config.Gemini
		cfg.Model = "gemini-2.5-flash"
		if r.isProviderHealthy(ProviderGemini) {
			return cfg
		}
		// Fallback: Claude Sonnet
		if r.isProviderHealthy(ProviderClaude) {
			return r.config.Claude
		}

	case SeverityP3Medium, SeverityP4Info:
		// P3/P4: Gemini Flash-Lite — cheapest, batch-friendly
		cfg := r.config.Gemini
		cfg.Model = "gemini-2.5-flash-lite"
		if r.isProviderHealthy(ProviderGemini) {
			return cfg
		}
	}

	// Ultimate fallback: Ollama local (always available)
	if r.isProviderHealthy(ProviderOllama) {
		r.logger.Warn("all cloud providers unavailable, falling back to Ollama")
		return r.config.Ollama
	}

	// Last resort: return default config
	return ProviderConfig{
		Provider: Provider(r.config.DefaultProvider),
		Model:    r.config.DefaultModel,
		Timeout:  30 * time.Second,
	}
}

// GetClient returns the LLM client for a provider.
func (r *Router) GetClient(provider Provider) (LLMClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, ok := r.clients[provider]
	if !ok {
		return nil, fmt.Errorf("no client registered for provider %s", provider)
	}
	return client, nil
}

// RecordFailure records a provider failure for circuit breaker logic.
func (r *Router) RecordFailure(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failures[provider]++
	r.lastFail[provider] = time.Now()

	r.logger.Warn("LLM provider failure recorded",
		"provider", provider,
		"consecutive_failures", r.failures[provider],
	)
}

// RecordSuccess resets the failure counter for a provider.
func (r *Router) RecordSuccess(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[provider] = 0
}

// isProviderHealthy checks if a provider is available (circuit breaker).
// A provider is considered unhealthy if it has 3+ consecutive failures
// in the last 5 minutes.
func (r *Router) isProviderHealthy(provider Provider) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.clients[provider]; !ok {
		return false // Not registered
	}

	failures := r.failures[provider]
	lastFail := r.lastFail[provider]

	// Circuit breaker: 3+ failures within last 5 minutes = unhealthy
	if failures >= 3 && time.Since(lastFail) < 5*time.Minute {
		return false
	}

	// Reset after 5 minutes (half-open state)
	if failures >= 3 && time.Since(lastFail) >= 5*time.Minute {
		return true // Allow one retry
	}

	return true
}

// AvailableProviders returns a list of registered and healthy providers.
func (r *Router) AvailableProviders() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var providers []Provider
	for p := range r.clients {
		if r.isProviderHealthy(p) {
			providers = append(providers, p)
		}
	}
	return providers
}
