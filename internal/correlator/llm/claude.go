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

// ClaudeClient implements LLMClient for Anthropic Claude (Sonnet/Opus).
// Used for P0 (Opus) and P1 (Sonnet) incidents requiring complex reasoning.
type ClaudeClient struct {
	config ProviderConfig
	client *http.Client
	logger *slog.Logger
}

// NewClaudeClient creates a Claude client.
func NewClaudeClient(config ProviderConfig, logger *slog.Logger) *ClaudeClient {
	return &ClaudeClient{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: logger,
	}
}

func (c *ClaudeClient) Provider() Provider { return ProviderClaude }
func (c *ClaudeClient) Model() string      { return c.config.Model }

func (c *ClaudeClient) Healthy(ctx context.Context) bool {
	// Simple health check — try a minimal API call
	return c.config.APIKey != ""
}

func (c *ClaudeClient) Correlate(ctx context.Context, bundle AnomalyBundle, prompt string) (*CorrelationResult, error) {
	start := time.Now()

	reqBody := claudeRequest{
		Model:     c.config.Model,
		MaxTokens: 1024,
		System:    buildSystemPrompt(),
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling claude request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating claude request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claude API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading claude response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("claude API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp claudeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing claude response: %w", err)
	}

	text := ""
	if len(apiResp.Content) > 0 {
		text = apiResp.Content[0].Text
	}

	result := parseCorrelationJSON(text)
	result.Provider = ProviderClaude
	result.Model = c.config.Model
	result.Latency = time.Since(start)
	result.TokensIn = apiResp.Usage.InputTokens
	result.TokensOut = apiResp.Usage.OutputTokens
	result.RawText = text

	return result, nil
}

type claudeRequest struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []claudeMessage  `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
