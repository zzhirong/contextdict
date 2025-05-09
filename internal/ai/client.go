package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
	"github.com/zzhirong/contextdict/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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
	tracer := otel.Tracer("github.com/zzhirong/contextdict/internal/ai")
	ctx, span := tracer.Start(ctx, "ai.Generate")
	defer span.End()

	span.SetAttributes(attribute.String("ai.model", dsc.cfg.Model))
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
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Printf("AI ChatCompletion error: %v\n", err)
		return "", fmt.Errorf("AI request failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		err := fmt.Errorf("AI returned empty response")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		log.Printf("AI returned empty response or choices. Response: %+v", resp)
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
