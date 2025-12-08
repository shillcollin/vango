package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/shillcollin/gai/prompts"
)

//go:embed prompts/*.tmpl
var embeddedPrompts embed.FS

type promptInfo struct {
	Text        string
	Name        string
	Version     string
	Fingerprint string
}

type promptAssets struct {
	Registry  *prompts.Registry
	System    promptInfo
	ToolLimit promptTemplate
}

type promptTemplate struct {
	Name        string
	Version     string
	Fingerprint string
}

func loadPromptAssets() (promptAssets, error) {
	opts := []prompts.RegistryOption{}
	if override := strings.TrimSpace(os.Getenv("GAI_PROMPTS_OVERRIDE_DIR")); override != "" {
		opts = append(opts, prompts.WithOverrideDir(override))
	}

	registry := prompts.NewRegistry(embeddedPrompts, opts...)
	if err := registry.Reload(); err != nil {
		return promptAssets{}, fmt.Errorf("reload prompts: %w", err)
	}

	data := map[string]any{
		"AppName": "GAI Web Demo",
	}

	rendered, id, err := registry.Render(context.Background(), "chat_system", "", data)
	if err != nil {
		return promptAssets{}, fmt.Errorf("render chat_system prompt: %w", err)
	}

	toolLimitPreview, toolLimitID, err := registry.Render(context.Background(), "tool_limit_finalizer", "", map[string]any{"Limit": 4})
	if err != nil {
		return promptAssets{}, fmt.Errorf("render tool_limit_finalizer prompt: %w", err)
	}
	_ = toolLimitPreview

	return promptAssets{
		Registry: registry,
		System: promptInfo{
			Text:        rendered,
			Name:        id.Name,
			Version:     id.Version,
			Fingerprint: id.Fingerprint,
		},
		ToolLimit: promptTemplate{
			Name:        toolLimitID.Name,
			Version:     toolLimitID.Version,
			Fingerprint: toolLimitID.Fingerprint,
		},
	}, nil
}
