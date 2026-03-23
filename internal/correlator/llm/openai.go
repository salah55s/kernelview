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

// OpenAIClient implements LLMClient for OpenAI GPT-4o.
// Specialized for APP family incidents (CrashLoopBackOff, code analysis, stack traces).
type OpenAIClient struct {
	config ProviderConfig
	client *http.Client
	logger *slog.Logger
}

// NewOpenAIClient creates an OpenAI client.
func NewOpenAIClient(config ProviderConfig, logger *slog.Logger) *OpenAIClient {
	return &OpenAIClient{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: logger,
	}
}

func (o *OpenAIClient) Provider() Provider { return ProviderOpenAI }
func (o *OpenAIClient) Model() string      { return o.config.Model }

func (o *OpenAIClient) Healthy(ctx context.Context) bool {
	return o.config.APIKey != ""
}

func (o *OpenAIClient) Correlate(ctx context.Context, bundle AnomalyBundle, prompt string) (*CorrelationResult, error) {
	start := time.Now()

	// OpenAI structured output via response_format JSON schema
	reqBody := map[string]interface{}{
		"model": o.config.Model,
		"messages": []map[string]string{
			{"role": "system", "content": buildSystemPrompt()},
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]interface{}{
			"type": "json_schema",
			"json_schema": map[string]interface{}{
				"name":   "CorrelationResult",
				"strict": true,
				"schema": openAICorrelationSchema(),
			},
		},
		"max_tokens": 1024,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating openai request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.config.APIKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading openai response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing openai response: %w", err)
	}

	text := ""
	if len(apiResp.Choices) > 0 {
		text = apiResp.Choices[0].Message.Content
	}

	result := parseCorrelationJSON(text)
	result.Provider = ProviderOpenAI
	result.Model = o.config.Model
	result.Latency = time.Since(start)
	result.TokensIn = apiResp.Usage.PromptTokens
	result.TokensOut = apiResp.Usage.CompletionTokens
	result.RawText = text

	return result, nil
}

// openAICorrelationSchema returns the JSON schema for OpenAI structured output.
func openAICorrelationSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"root_cause":            map[string]interface{}{"type": "string"},
			"trigger":               map[string]interface{}{"type": "string"},
			"confidence":            map[string]interface{}{"type": "string", "enum": []string{"HIGH", "MEDIUM", "LOW"}},
			"severity":              map[string]interface{}{"type": "string", "enum": []string{"P0", "P1", "P2", "P3"}},
			"incident_class":        map[string]interface{}{"type": "string"},
			"recommended_action":    map[string]interface{}{"type": "string"},
			"auto_remediation_safe": map[string]interface{}{"type": "boolean"},
			"action_type":           map[string]interface{}{"type": "string"},
			"incident_subtype":      map[string]interface{}{"type": "string"},
			"evidence_from_logs":    map[string]interface{}{"type": "string"},
			"evidence_from_syscalls": map[string]interface{}{"type": "string"},
			"estimated_fix_time_minutes": map[string]interface{}{"type": "integer"},
			"human_required":        map[string]interface{}{"type": "boolean"},
		},
		"required":             []string{"root_cause", "confidence", "severity", "auto_remediation_safe"},
		"additionalProperties": false,
	}
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}
