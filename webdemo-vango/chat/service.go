// Package chat provides the AI chat backend service.
package chat

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shillcollin/gai/core"
	"github.com/shillcollin/gai/providers/anthropic"
	"github.com/shillcollin/gai/providers/gemini"
	"github.com/shillcollin/gai/providers/groq"
	openai "github.com/shillcollin/gai/providers/openai"
	openairesponses "github.com/shillcollin/gai/providers/openai-responses"
	"github.com/shillcollin/gai/providers/xai"
	"github.com/shillcollin/gai/runner"
)

// ProviderInfo describes an AI provider's capabilities.
type ProviderInfo struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	DefaultModel string   `json:"default_model"`
	Models       []string `json:"models"`
	Tools        []string `json:"tools"`
	Capabilities Caps     `json:"capabilities"`
}

// Caps represents provider capabilities.
type Caps struct {
	Streaming bool `json:"streaming"`
	Images    bool `json:"images"`
	Tools     bool `json:"tools"`
}

// Message represents a chat message.
type Message struct {
	ID           string      `json:"id"`
	Role         string      `json:"role"`
	Content      string      `json:"content"`
	Parts        []Part      `json:"parts,omitempty"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	Reasoning    []Reasoning `json:"reasoning,omitempty"`
	Usage        *Usage      `json:"usage,omitempty"`
	Status       string      `json:"status"`
	Provider     string      `json:"provider,omitempty"`
	Model        string      `json:"model,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
	CreatedAt    int64       `json:"created_at"`
}

// Part is a message content part.
type Part struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	DataURL string `json:"data_url,omitempty"`
	Data    string `json:"data,omitempty"`
	Mime    string `json:"mime,omitempty"`
}

// ToolCall represents a tool invocation.
type ToolCall struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Result     any            `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Retries    int            `json:"retries,omitempty"`
}

// Reasoning represents a reasoning trace entry.
type Reasoning struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // "thinking" or "summary"
	Text string `json:"text"`
	Step int    `json:"step,omitempty"`
}

// Usage tracks token usage.
type Usage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	TotalTokens     int `json:"total_tokens"`
}

// ChatRequest is the request payload for chat.
type ChatRequest struct {
	Provider    string    `json:"provider"`
	Model       string    `json:"model,omitempty"`
	Mode        string    `json:"mode,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature,omitempty"`
	ToolChoice  string    `json:"tool_choice,omitempty"`
	Tools       []string  `json:"tools,omitempty"`
}

// StreamEvent is a streaming response event.
type StreamEvent struct {
	Type             string    `json:"type"`
	Step             int       `json:"step,omitempty"`
	TextDelta        string    `json:"text_delta,omitempty"`
	ReasoningDelta   string    `json:"reasoning_delta,omitempty"`
	ReasoningSummary string    `json:"reasoning_summary,omitempty"`
	ToolCall         *ToolCall `json:"tool_call,omitempty"`
	ToolResult       *ToolCall `json:"tool_result,omitempty"`
	Usage            *Usage    `json:"usage,omitempty"`
	Provider         string    `json:"provider,omitempty"`
	Model            string    `json:"model,omitempty"`
	FinishReason     string    `json:"finish_reason,omitempty"`
}

// providerEntry holds a provider client and its metadata.
type providerEntry struct {
	Label        string
	DefaultModel string
	Models       []string
	Client       core.Provider
}

// Service manages AI provider connections.
type Service struct {
	providers map[string]providerEntry
}

// NewService creates a new chat service with available providers.
func NewService() (*Service, error) {
	providers := make(map[string]providerEntry)

	if key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY")); key != "" {
		chatClient := openai.New(
			openai.WithAPIKey(key),
			openai.WithModel("gpt-4.1-mini"),
		)
		providers["openai-chat"] = providerEntry{
			Label:        "OpenAI Chat",
			DefaultModel: "gpt-4.1-mini",
			Models:       []string{"gpt-4.1-mini", "gpt-4.1", "gpt-4o"},
			Client:       chatClient,
		}

		responsesClient := openairesponses.New(
			openairesponses.WithAPIKey(key),
			openairesponses.WithModel("o4-mini"),
		)
		providers["openai-responses"] = providerEntry{
			Label:        "OpenAI Responses",
			DefaultModel: "o4-mini",
			Models:       []string{"o4-mini", "o4", "gpt-4.1-mini"},
			Client:       responsesClient,
		}
	}

	if key := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")); key != "" {
		client := anthropic.New(
			anthropic.WithAPIKey(key),
			anthropic.WithModel("claude-3-7-sonnet-20250219"),
		)
		providers["anthropic"] = providerEntry{
			Label:        "Anthropic",
			DefaultModel: "claude-3-7-sonnet-20250219",
			Models:       []string{"claude-sonnet-4-20250514", "claude-3-5-haiku-20241022"},
			Client:       client,
		}
	}

	if key := strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")); key != "" {
		client := gemini.New(
			gemini.WithAPIKey(key),
			gemini.WithModel("gemini-2.5-pro"),
		)
		providers["gemini"] = providerEntry{
			Label:        "Gemini",
			DefaultModel: "gemini-2.5-pro",
			Models:       []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite"},
			Client:       client,
		}
	}

	if key := strings.TrimSpace(os.Getenv("GROQ_API_KEY")); key != "" {
		client := groq.New(
			groq.WithAPIKey(key),
			groq.WithModel("llama-3.1-8b-instant"),
		)
		providers["groq"] = providerEntry{
			Label:        "Groq",
			DefaultModel: "llama-3.1-8b-instant",
			Models:       []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant"},
			Client:       client,
		}
	}

	if key := strings.TrimSpace(os.Getenv("XAI_API_KEY")); key != "" {
		client := xai.New(
			xai.WithAPIKey(key),
			xai.WithModel("grok-4"),
		)
		providers["xai"] = providerEntry{
			Label:        "XAI",
			DefaultModel: "grok-4",
			Models:       []string{"grok-4", "grok-3"},
			Client:       client,
		}
	}

	if len(providers) == 0 {
		return nil, errors.New("no API keys configured; set OPENAI_API_KEY, ANTHROPIC_API_KEY, GOOGLE_API_KEY, GROQ_API_KEY, or XAI_API_KEY")
	}

	return &Service{providers: providers}, nil
}

// Providers returns available providers.
func (s *Service) Providers() []ProviderInfo {
	list := make([]ProviderInfo, 0, len(s.providers))
	for id, p := range s.providers {
		caps := p.Client.Capabilities()
		list = append(list, ProviderInfo{
			ID:           id,
			Label:        p.Label,
			DefaultModel: p.DefaultModel,
			Models:       p.Models,
			Tools:        []string{}, // TODO: add tavily tools
			Capabilities: Caps{
				Streaming: caps.Streaming,
				Images:    caps.Images,
				Tools:     true, // Most providers support tools
			},
		})
	}
	return list
}

// GetProvider returns a provider by ID.
func (s *Service) GetProvider(id string) (ProviderInfo, bool) {
	p, ok := s.providers[id]
	if !ok {
		return ProviderInfo{}, false
	}
	caps := p.Client.Capabilities()
	return ProviderInfo{
		ID:           id,
		Label:        p.Label,
		DefaultModel: p.DefaultModel,
		Models:       p.Models,
		Capabilities: Caps{
			Streaming: caps.Streaming,
			Images:    caps.Images,
			Tools:     true, // Most providers support tools
		},
	}, true
}

// SendMessage sends a chat message and streams the response via channel.
func (s *Service) SendMessage(ctx context.Context, req ChatRequest, eventCh chan<- StreamEvent) error {
	defer close(eventCh)

	entry, ok := s.providers[req.Provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", req.Provider)
	}

	// Convert messages to core format
	messages, err := convertMessages(req.Messages)
	if err != nil {
		return fmt.Errorf("converting messages: %w", err)
	}

	// Build core request
	model := req.Model
	if model == "" {
		model = entry.DefaultModel
	}

	coreReq := core.Request{
		Model:       model,
		Messages:    messages,
		Temperature: req.Temperature,
	}

	// Send start event
	eventCh <- StreamEvent{
		Type:     "start",
		Provider: entry.Label,
		Model:    model,
	}

	// Create runner and stream
	run := runner.New(entry.Client,
		runner.WithOnToolError(runner.ToolErrorAppendAndContinue),
		runner.WithToolTimeout(25*time.Second),
	)

	stream, err := run.StreamRequest(ctx, coreReq)
	if err != nil {
		eventCh <- StreamEvent{Type: "error"}
		return fmt.Errorf("starting stream: %w", err)
	}
	defer stream.Close()

	var usageTracker *Usage

	// Process stream events
	for event := range stream.Events() {
		switch event.Type {
		case core.EventTextDelta:
			eventCh <- StreamEvent{
				Type:      "text.delta",
				TextDelta: event.TextDelta,
				Step:      event.StepID,
			}
		case core.EventReasoningDelta:
			eventCh <- StreamEvent{
				Type:           "reasoning.delta",
				ReasoningDelta: event.ReasoningDelta,
				Step:           event.StepID,
			}
		case core.EventReasoningSummary:
			eventCh <- StreamEvent{
				Type:             "reasoning.summary",
				ReasoningSummary: event.ReasoningSummary,
				Step:             event.StepID,
			}
		case core.EventToolCall:
			eventCh <- StreamEvent{
				Type: "tool.call",
				Step: event.StepID,
				ToolCall: &ToolCall{
					ID:     event.ToolCall.ID,
					Name:   event.ToolCall.Name,
					Status: "running",
					Input:  event.ToolCall.Input,
				},
			}
		case core.EventToolResult:
			eventCh <- StreamEvent{
				Type: "tool.result",
				Step: event.StepID,
				ToolResult: &ToolCall{
					ID:     event.ToolResult.ID,
					Name:   event.ToolResult.Name,
					Status: "completed",
					Result: event.ToolResult.Result,
					Error:  event.ToolResult.Error,
				},
			}
		case core.EventFinish:
			usageTracker = &Usage{
				InputTokens:  event.Usage.InputTokens,
				OutputTokens: event.Usage.OutputTokens,
				TotalTokens:  event.Usage.TotalTokens,
			}
			finishReason := ""
			if event.FinishReason != nil {
				finishReason = string(event.FinishReason.Type)
			}
			eventCh <- StreamEvent{
				Type:         "finish",
				Usage:        usageTracker,
				FinishReason: finishReason,
			}
		}
	}

	if err := stream.Err(); err != nil && !errors.Is(err, core.ErrStreamClosed) {
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// convertMessages converts API messages to core format.
func convertMessages(msgs []Message) ([]core.Message, error) {
	converted := make([]core.Message, 0, len(msgs))
	for _, msg := range msgs {
		role := core.Role(strings.ToLower(strings.TrimSpace(msg.Role)))
		switch role {
		case core.System, core.User, core.Assistant:
		default:
			return nil, fmt.Errorf("unsupported role %s", msg.Role)
		}

		// If we have parts, convert them
		if len(msg.Parts) > 0 {
			parts := make([]core.Part, 0, len(msg.Parts))
			for _, part := range msg.Parts {
				switch strings.ToLower(part.Type) {
				case "text":
					parts = append(parts, core.Text{Text: part.Text})
				case "image", "image_base64":
					data := part.Data
					if data == "" && part.DataURL != "" {
						// Extract base64 from data URL
						if idx := strings.Index(part.DataURL, ","); idx > 0 {
							data = part.DataURL[idx+1:]
						}
					}
					decoded, err := base64.StdEncoding.DecodeString(data)
					if err != nil {
						return nil, fmt.Errorf("invalid image data: %w", err)
					}
					mime := part.Mime
					if mime == "" {
						mime = "image/png"
					}
					parts = append(parts, core.Image{Source: core.BlobRef{Kind: core.BlobBytes, Bytes: decoded, MIME: mime, Size: int64(len(decoded))}})
				default:
					return nil, fmt.Errorf("unsupported part type %s", part.Type)
				}
			}
			converted = append(converted, core.Message{Role: role, Parts: parts})
		} else {
			// Simple text message
			converted = append(converted, core.Message{
				Role:  role,
				Parts: []core.Part{core.Text{Text: msg.Content}},
			})
		}
	}
	return converted, nil
}

// NewMessage creates a new message with a unique ID.
func NewMessage(role, content string) Message {
	return Message{
		ID:        uuid.NewString(),
		Role:      role,
		Content:   content,
		Status:    "complete",
		CreatedAt: time.Now().UnixMilli(),
	}
}
