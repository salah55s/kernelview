package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// OllamaClient implements LLMClient for local Ollama inference.
// Used for BYOC enterprise customers with data sovereignty requirements
// (banking, government, healthcare). Data never leaves the cluster.
type OllamaClient struct {
	config ProviderConfig
	client *http.Client
	logger *slog.Logger
}

// NewOllamaClient creates an Ollama client.
// Default endpoint: http://ollama.kernelview-system.svc:11434
func NewOllamaClient(config ProviderConfig, logger *slog.Logger) *OllamaClient {
	if config.Endpoint == "" {
		config.Endpoint = "http://ollama.kernelview-system.svc:11434"
	}
	if config.Timeout == 0 {
		// Ollama inference is slower — 120s timeout vs 30s for cloud APIs
		config.Timeout = 120 * time.Second
	}

	return &OllamaClient{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: logger,
	}
}

func (o *OllamaClient) Provider() Provider { return ProviderOllama }
func (o *OllamaClient) Model() string      { return o.config.Model }

func (o *OllamaClient) Healthy(ctx context.Context) bool {
	url := fmt.Sprintf("%s/api/version", o.config.Endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (o *OllamaClient) Correlate(ctx context.Context, bundle AnomalyBundle, prompt string) (*CorrelationResult, error) {
	start := time.Now()

	// Ollama exposes an OpenAI-compatible API
	url := fmt.Sprintf("%s/v1/chat/completions", o.config.Endpoint)

	reqBody := map[string]interface{}{
		"model": o.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": buildSystemPrompt()},
			{"role": "user", "content": prompt},
		},
		"format":      "json",  // Ollama's native JSON mode
		"stream":      false,
		"temperature": 0.1,     // Low temp for consistent structured output
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ollama response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	// OpenAI-compatible response format
	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing ollama response: %w", err)
	}

	text := ""
	if len(apiResp.Choices) > 0 {
		text = apiResp.Choices[0].Message.Content
	}

	result := parseCorrelationJSON(text)
	result.Provider = ProviderOllama
	result.Model = o.config.Model
	result.Latency = time.Since(start)
	result.RawText = text

	return result, nil
}
