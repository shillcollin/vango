package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/vdom"
)

type InputConfig struct {
	BaseConfig
	Type        string
	Placeholder string
	Value       string
}

func (c *InputConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type InputOption = Option[*InputConfig]

func InputType(t string) InputOption {
	return func(c *InputConfig) { c.Type = t }
}

func InputPlaceholder(s string) InputOption {
	return func(c *InputConfig) { c.Placeholder = s }
}

func InputValue(s string) InputOption {
	return func(c *InputConfig) { c.Value = s }
}

func Input(opts ...InputOption) *vdom.VNode {
	c := &InputConfig{
		Type: "text", // Default
	}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-base ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+4)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	if c.Type != "" {
		renderOpts = append(renderOpts, vdom.Type(c.Type))
	}
	if c.Placeholder != "" {
		renderOpts = append(renderOpts, vdom.Placeholder(c.Placeholder))
	}
	if c.Value != "" {
		renderOpts = append(renderOpts, vdom.Value(c.Value))
	}
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Input(renderOpts...)
}
