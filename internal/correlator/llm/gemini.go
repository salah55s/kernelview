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

// GeminiClient implements LLMClient for Google Gemini.
// Used for P2 (Flash) and P3 (Flash-Lite) incidents.
// Key advantage: 1M token context window and native JSON schema enforcement.
type GeminiClient struct {
	config ProviderConfig
	client *http.Client
	logger *slog.Logger
}

// NewGeminiClient creates a Gemini client.
func NewGeminiClient(config ProviderConfig, logger *slog.Logger) *GeminiClient {
	return &GeminiClient{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
		logger: logger,
	}
}

func (g *GeminiClient) Provider() Provider { return ProviderGemini }
func (g *GeminiClient) Model() string      { return g.config.Model }

func (g *GeminiClient) Healthy(ctx context.Context) bool {
	return g.config.APIKey != ""
}

func (g *GeminiClient) Correlate(ctx context.Context, bundle AnomalyBundle, prompt string) (*CorrelationResult, error) {
	start := time.Now()

	// Gemini uses the generativelanguage API with native JSON schema enforcement
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: buildSystemPrompt() + "\n\n" + prompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			ResponseMIMEType: "application/json",
			ResponseSchema:   correlationResultSchema(),
			MaxOutputTokens:  1024,
			Temperature:      0.1,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling gemini request: %w", err)
	}

	// Gemini API endpoint uses model name in URL
	// IMPORTANT: Use gemini-2.5-flash, NOT 2.0 (retired June 2026)
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.config.Model, g.config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading gemini response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResp geminiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing gemini response: %w", err)
	}

	text := ""
	if len(apiResp.Candidates) > 0 && len(apiResp.Candidates[0].Content.Parts) > 0 {
		text = apiResp.Candidates[0].Content.Parts[0].Text
	}

	// Gemini with ResponseSchema returns valid JSON — parse directly
	result := parseCorrelationJSON(text)
	result.Provider = ProviderGemini
	result.Model = g.config.Model
	result.Latency = time.Since(start)
	if apiResp.UsageMetadata != nil {
		result.TokensIn = apiResp.UsageMetadata.PromptTokenCount
		result.TokensOut = apiResp.UsageMetadata.CandidatesTokenCount
	}
	result.RawText = text

	return result, nil
}

// correlationResultSchema returns the JSON schema for Gemini's native enforcement.
// This is the key Gemini advantage: structured output without prompt engineering.
func correlationResultSchema() *geminiSchema {
	return &geminiSchema{
		Type: "OBJECT",
		Properties: map[string]*geminiSchema{
			"root_cause":            {Type: "STRING"},
			"trigger":               {Type: "STRING"},
			"confidence":            {Type: "STRING", Enum: []string{"HIGH", "MEDIUM", "LOW"}},
			"severity":              {Type: "STRING", Enum: []string{"P0", "P1", "P2", "P3"}},
			"incident_class":        {Type: "STRING"},
			"recommended_action":    {Type: "STRING"},
			"auto_remediation_safe": {Type: "BOOLEAN"},
			"action_type":           {Type: "STRING"},
		},
		Required: []string{"root_cause", "confidence", "severity", "auto_remediation_safe"},
	}
}

// Gemini API types
type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	ResponseMIMEType string        `json:"responseMimeType,omitempty"`
	ResponseSchema   *geminiSchema `json:"responseSchema,omitempty"`
	MaxOutputTokens  int           `json:"maxOutputTokens,omitempty"`
	Temperature      float64       `json:"temperature,omitempty"`
}

type geminiSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]*geminiSchema  `json:"properties,omitempty"`
	Required   []string                  `json:"required,omitempty"`
	Enum       []string                  `json:"enum,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}
