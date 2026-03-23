// Package scrubber provides secrets detection and redaction for log data
// before sending to external LLM APIs.
package scrubber

import (
	"regexp"
	"strings"
)

// Scrubber detects and redacts sensitive data from text.
type Scrubber struct {
	enabled  bool
	patterns []*pattern
}

type pattern struct {
	name    string
	regex   *regexp.Regexp
	replace string
}

// New creates a secrets scrubber with default patterns.
func New(enabled bool) *Scrubber {
	s := &Scrubber{enabled: enabled}
	if enabled {
		s.patterns = defaultPatterns()
	}
	return s
}

// Scrub removes sensitive data from the input text.
func (s *Scrubber) Scrub(text string) string {
	if !s.enabled || text == "" {
		return text
	}

	result := text
	for _, p := range s.patterns {
		result = p.regex.ReplaceAllString(result, p.replace)
	}
	return result
}

// ScrubLines scrubs each line individually.
func (s *Scrubber) ScrubLines(lines []string) []string {
	if !s.enabled {
		return lines
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = s.Scrub(line)
	}
	return result
}

// ContainsSensitiveData returns true if the text likely contains secrets.
func (s *Scrubber) ContainsSensitiveData(text string) bool {
	if !s.enabled {
		return false
	}
	for _, p := range s.patterns {
		if p.regex.MatchString(text) {
			return true
		}
	}
	return false
}

// defaultPatterns returns regex patterns for common secret formats.
func defaultPatterns() []*pattern {
	return []*pattern{
		// AWS Access Key ID (starts with AKIA)
		{
			name:    "aws_access_key",
			regex:   regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			replace: "[REDACTED:AWS_KEY]",
		},
		// AWS Secret Access Key (40 chars, base64-like)
		{
			name:    "aws_secret_key",
			regex:   regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*[A-Za-z0-9/+=]{40}`),
			replace: "[REDACTED:AWS_SECRET]",
		},
		// JWT Tokens (3 base64url segments separated by dots)
		{
			name:    "jwt_token",
			regex:   regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
			replace: "[REDACTED:JWT]",
		},
		// Generic API keys (common patterns)
		{
			name:    "api_key_header",
			regex:   regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?secret|api[_-]?token)\s*[=:]\s*[A-Za-z0-9_\-]{20,}`),
			replace: "[REDACTED:API_KEY]",
		},
		// Bearer tokens
		{
			name:    "bearer_token",
			regex:   regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_\-\.]{20,}`),
			replace: "[REDACTED:BEARER_TOKEN]",
		},
		// Credit card numbers (basic Luhn-eligible patterns)
		{
			name:    "credit_card",
			regex:   regexp.MustCompile(`\b[0-9]{4}[\s-]?[0-9]{4}[\s-]?[0-9]{4}[\s-]?[0-9]{4}\b`),
			replace: "[REDACTED:CARD_NUMBER]",
		},
		// Email addresses
		{
			name:    "email",
			regex:   regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			replace: "[REDACTED:EMAIL]",
		},
		// Private keys (PEM format)
		{
			name:    "private_key",
			regex:   regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----[\s\S]*?-----END\s+(RSA\s+)?PRIVATE\s+KEY-----`),
			replace: "[REDACTED:PRIVATE_KEY]",
		},
		// Password in URLs
		{
			name:    "url_password",
			regex:   regexp.MustCompile(`://[^:]+:[^@]+@`),
			replace: "://[REDACTED]@",
		},
		// Generic password patterns
		{
			name:    "password_field",
			regex:   regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[=:]\s*\S+`),
			replace: "[REDACTED:PASSWORD]",
		},
		// Google Cloud service account key
		{
			name:    "gcp_key",
			regex:   regexp.MustCompile(`"private_key":\s*"-----BEGIN`),
			replace: `"private_key": "[REDACTED:GCP_KEY]`,
		},
		// GitHub personal access tokens
		{
			name:    "github_token",
			regex:   regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,}`),
			replace: "[REDACTED:GITHUB_TOKEN]",
		},
		// Slack tokens
		{
			name:    "slack_token",
			regex:   regexp.MustCompile(`xox[baprs]-[0-9]+-[0-9]+-[A-Za-z0-9]+`),
			replace: "[REDACTED:SLACK_TOKEN]",
		},
	}
}

// SanitizeForUTF8 converts non-UTF-8 bytes to valid UTF-8
// with replacement characters. Required for Java services
// that emit binary log data.
func SanitizeForUTF8(data []byte) string {
	return strings.ToValidUTF8(string(data), "�")
}
