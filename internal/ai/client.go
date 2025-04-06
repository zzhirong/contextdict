package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
	"github.com/zzhirong/contextdict/config"
)

type Client interface {
	Generate(ctx context.Context, prompt string, texts ...string) (string, error)
}

type DeepSeekClient struct {
	client *openai.Client
	cfg    config.AIConfig
}

func NewClient(cfg config.AIConfig) Client {
	oaiConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		oaiConfig.BaseURL = cfg.BaseURL
	}

	client := openai.NewClientWithConfig(oaiConfig)
	return &DeepSeekClient{
		client: client,
		cfg:    cfg,
	}
}

func (dsc *DeepSeekClient) Generate(ctx context.Context, prompt string, texts ...string) (string, error) {
	messages := make([]openai.ChatCompletionMessage, len(texts)+1)
	messages[0] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: prompt,
	}
	for i, text := range texts {
		messages[i+1] = openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: text,
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    dsc.cfg.Model,
		Messages: messages,
	}

	resp, err := dsc.client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("AI ChatCompletion error: %v\n", err)
		return "", fmt.Errorf("AI request failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		log.Printf("AI returned empty response or choices. Response: %+v", resp)
		return "", fmt.Errorf("AI returned empty response")
	}

	return resp.Choices[0].Message.Content, nil
}
