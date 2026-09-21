package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Request struct {
	Prompt, Model string
	Temperature   float64
	Output        map[string]interface{}
}
type Provider interface {
	Complete(context.Context, Request) (string, error)
	Stream(context.Context, Request) (<-chan string, <-chan error)
}

type MockProvider struct{ Response string }

func (m MockProvider) Complete(_ context.Context, r Request) (string, error) {
	if m.Response != "" {
		return m.Response, nil
	}
	if r.Output != nil {
		b, _ := json.Marshal(map[string]interface{}{"ok": true, "prompt": r.Prompt})
		return string(b), nil
	}
	return "MOCK: " + r.Prompt, nil
}
func (m MockProvider) Stream(ctx context.Context, r Request) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		s, e := m.Complete(ctx, r)
		if e != nil {
			errs <- e
			return
		}
		for _, x := range strings.Fields(s) {
			select {
			case out <- x:
			case <-ctx.Done():
				errs <- ctx.Err()
				return
			}
		}
	}()
	return out, errs
}

type OpenAIProvider struct {
	APIKey, BaseURL string
	Client          *http.Client
}

func NewOpenAI() OpenAIProvider {
	base := os.Getenv("OPENAI_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return OpenAIProvider{APIKey: os.Getenv("OPENAI_API_KEY"), BaseURL: strings.TrimRight(base, "/"), Client: &http.Client{}}
}
func (p OpenAIProvider) Complete(ctx context.Context, r Request) (string, error) {
	if p.APIKey == "" {
		return "", errors.New("PROVIDER_ERROR: OPENAI_API_KEY is not set")
	}
	body := map[string]interface{}{"model": r.Model, "messages": []map[string]string{{"role": "user", "content": r.Prompt}}, "temperature": r.Temperature}
	if body["model"] == "" {
		body["model"] = "gpt-4o-mini"
	}
	if r.Output != nil {
		body["response_format"] = map[string]interface{}{"type": "json_schema", "json_schema": map[string]interface{}{"name": "ail_output", "schema": r.Output, "strict": true}}
	}
	b, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewReader(b))
	if e != nil {
		return "", fmt.Errorf("PROVIDER_ERROR: %w", e)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, e := p.Client.Do(req)
	if e != nil {
		return "", fmt.Errorf("PROVIDER_ERROR: %w", e)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("PROVIDER_ERROR: HTTP %d: %s", resp.StatusCode, string(data))
	}
	var x struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if e = json.Unmarshal(data, &x); e != nil {
		return "", fmt.Errorf("PROVIDER_ERROR: invalid response: %w", e)
	}
	if len(x.Choices) == 0 {
		return "", errors.New("PROVIDER_ERROR: response has no choices")
	}
	return x.Choices[0].Message.Content, nil
}
func (p OpenAIProvider) Stream(ctx context.Context, r Request) (<-chan string, <-chan error) {
	out := make(chan string)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		if p.APIKey == "" {
			errs <- errors.New("PROVIDER_ERROR: OPENAI_API_KEY is not set")
			return
		}
		body := map[string]interface{}{"model": r.Model, "messages": []map[string]string{{"role": "user", "content": r.Prompt}}, "temperature": r.Temperature, "stream": true}
		if body["model"] == "" {
			body["model"] = "gpt-4o-mini"
		}
		b, _ := json.Marshal(body)
		req, e := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewReader(b))
		if e != nil {
			errs <- fmt.Errorf("PROVIDER_ERROR: %w", e)
			return
		}
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		req.Header.Set("Content-Type", "application/json")
		resp, e := p.Client.Do(req)
		if e != nil {
			errs <- fmt.Errorf("PROVIDER_ERROR: %w", e)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			d, _ := io.ReadAll(resp.Body)
			errs <- fmt.Errorf("PROVIDER_ERROR: HTTP %d: %s", resp.StatusCode, string(d))
			return
		}
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := strings.TrimPrefix(sc.Text(), "data: ")
			if line == "[DONE]" {
				break
			}
			var x struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(line), &x) == nil && len(x.Choices) > 0 && x.Choices[0].Delta.Content != "" {
				select {
				case out <- x.Choices[0].Delta.Content:
				case <-ctx.Done():
					errs <- ctx.Err()
					return
				}
			}
		}
		if e := sc.Err(); e != nil {
			errs <- fmt.Errorf("PROVIDER_ERROR: %w", e)
		}
	}()
	return out, errs
}
