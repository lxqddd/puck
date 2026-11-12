package ai

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type GeminiProvider struct {
	APIKey  string
	BaseURL string
	Model   string
}

func (p *GeminiProvider) Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error {
	model := strings.TrimSpace(p.Model)
	if model == "" {
		return fmt.Errorf("gemini model is empty")
	}

	clientConfig := &genai.ClientConfig{APIKey: p.APIKey, Backend: genai.BackendGeminiAPI}
	if baseURL := strings.TrimSpace(p.BaseURL); baseURL != "" {
		clientConfig.HTTPOptions = genai.HTTPOptions{BaseURL: baseURL}
	}
	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return fmt.Errorf("gemini client: %w", err)
	}

	contents := make([]*genai.Content, 0, len(req.Messages))
	for _, message := range req.Messages {
		var role genai.Role = genai.RoleUser
		if message.Role == RoleAssistant {
			role = genai.RoleModel
		}
		contents = append(contents, genai.NewContentFromText(message.Content, role))
	}

	config := &genai.GenerateContentConfig{MaxOutputTokens: 4096}
	if system := strings.TrimSpace(req.System); system != "" {
		config.SystemInstruction = genai.NewContentFromText(system, genai.RoleUser)
	}

	for response, err := range client.Models.GenerateContentStream(ctx, model, contents, config) {
		if err != nil {
			return fmt.Errorf("gemini stream: %w", err)
		}
		text := response.Text()
		if text == "" {
			continue
		}
		if err := onEvent(ChatEvent{Type: EventDelta, Text: text}); err != nil {
			return err
		}
	}
	return nil
}
