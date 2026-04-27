package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	apiURL     = "https://api.anthropic.com/v1/messages"
	apiVersion = "2023-06-01"
	maxTokens  = 1024
)

var ErrEmptyResponse = errors.New("claude: empty response")

type Client struct {
	http   *http.Client
	apiKey string
	model  string
}

func New(apiKey, model string) *Client {
	return &Client{
		http:   &http.Client{Timeout: 60 * time.Second},
		apiKey: apiKey,
		model:  model,
	}
}

// Message is one turn in a conversation. Role is "user" or "assistant".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type cacheControl struct {
	Type string `json:"type"`
}

type systemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type request struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	System    []systemBlock `json:"system,omitempty"`
	Messages  []Message     `json:"messages"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type response struct {
	Content []contentBlock `json:"content"`
}

// Advise sends a single user message with an optional system prompt.
func (c *Client) Advise(ctx context.Context, system, user string) (string, error) {
	return c.Chat(ctx, system, []Message{{Role: "user", Content: user}})
}

// Chat sends a multi-turn conversation. The system prompt is sent as a cached
// block so resending it across turns is cheap.
func (c *Client) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	req := request{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages:  messages,
	}
	if system != "" {
		req.System = []systemBlock{{
			Type:         "text",
			Text:         system,
			CacheControl: &cacheControl{Type: "ephemeral"},
		}}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("claude HTTP %d: %s", resp.StatusCode, buf)
	}

	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("decode claude response: %w", err)
	}

	var out bytes.Buffer
	for _, b := range r.Content {
		if b.Type == "text" {
			out.WriteString(b.Text)
		}
	}
	if out.Len() == 0 {
		return "", ErrEmptyResponse
	}
	return out.String(), nil
}
