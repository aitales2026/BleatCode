package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	openai "github.com/sashabaranov/go-openai"
)

const (
	defaultMaxTokens = 8000
	systemPrompt     = "You are a coding agent. Your name is BleatCode. Act, don't explain.\n\nWorking directory: %s"
)

// LoopConfig holds configuration for the agent loop.
type LoopConfig struct {
	WorkDir string
}

// Usage tracks token usage for a turn.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Loop is the main agent orchestration loop.
type Loop struct {
	client *openai.Client
	model  string
	config LoopConfig

	mu       sync.Mutex
	messages []openai.ChatCompletionMessage
	usage    Usage
}

// NewLoop creates a new agent loop.
func NewLoop(client *openai.Client, model string, config LoopConfig) *Loop {
	return &Loop{
		client: client,
		model:  model,
		config: config,
	}
}

// GetUsage returns the token usage from the last completed turn.
func (l *Loop) GetUsage() Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.usage
}

// Run executes one user turn with streaming output to the channel.
// The channel is closed when the turn is complete.
func (l *Loop) Run(ctx context.Context, query string, out chan<- string) {
	defer close(out)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.messages = append(l.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: query,
	})

	fullContent, usage, err := l.callLLMStream(ctx, fmt.Sprintf(systemPrompt, l.config.WorkDir), defaultMaxTokens, out)
	if err != nil {
		errMsg := fmt.Sprintf("[Error] %v", err)
		l.messages = append(l.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: errMsg,
		})
		out <- errMsg
		return
	}

	l.usage = usage

	l.messages = append(l.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleAssistant,
		Content: fullContent,
	})
}

// callLLMStream calls the LLM with streaming, sending content chunks to out.
// Returns the full accumulated content and token usage.
func (l *Loop) callLLMStream(ctx context.Context, system string, maxTokens int, out chan<- string) (string, Usage, error) {
	msgs := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: system},
	}
	msgs = append(msgs, l.messages...)

	req := openai.ChatCompletionRequest{
		Model:     l.model,
		Messages:  msgs,
		MaxTokens: maxTokens,
		Stream:    true,
	}

	stream, err := l.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return "", Usage{}, err
	}
	defer stream.Close()

	var fullContent strings.Builder
	var usage Usage

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fullContent.String(), usage, err
		}

		if chunk.Usage != nil {
			usage = Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fullContent.WriteString(choice.Delta.Content)
				out <- choice.Delta.Content
			}
		}
	}

	return fullContent.String(), usage, nil
}
