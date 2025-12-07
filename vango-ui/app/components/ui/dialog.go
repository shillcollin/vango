package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/features/hooks"
	"github.com/vango-dev/vango/v2/pkg/vango"
	"github.com/vango-dev/vango/v2/pkg/vdom"
)

// Constants
const (
	HookNameDialog = "Dialog"
	HookEventClose = "close"
)

// 1. Component Config
type DialogConfig struct {
	BaseConfig
	Open          *vango.Signal[bool]
	OnClose       func()
	CloseOnEscape bool
}

// 2. Hook Config (Wire Protocol)
type dialogHookConfig struct {
	Open          bool `json:"open"`
	CloseOnEscape bool `json:"closeOnEscape"`
}

// 3. Mapping Function
func makeDialogHookConfig(c *DialogConfig) dialogHookConfig {
	isOpen := false
	if c.Open != nil {
		isOpen = c.Open.Get()
	}
	return dialogHookConfig{
		Open:          isOpen,
		CloseOnEscape: c.CloseOnEscape,
	}
}

// Implement ConfigProvider interface
func (c *DialogConfig) GetBase() *BaseConfig { return &c.BaseConfig }

// Options
type DialogOption = Option[*DialogConfig]

func DialogOpen(s *vango.Signal[bool]) DialogOption {
	return func(c *DialogConfig) { c.Open = s }
}

func DialogOnClose(f func()) DialogOption {
	return func(c *DialogConfig) { c.OnClose = f }
}

func DialogCloseOnEscape(b bool) DialogOption {
	return func(c *DialogConfig) { c.CloseOnEscape = b }
}

// 4. Implementation
func Dialog(opts ...DialogOption) *vdom.VNode {
	c := &DialogConfig{} // Defaults
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"fixed left-[50%] top-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border bg-background p-6 shadow-lg duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%] sm:rounded-lg",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	// Hook attribute
	hookAttr := hooks.Hook(HookNameDialog, makeDialogHookConfig(c))

	// Close Handler
	closeHandler := hooks.OnEvent(HookEventClose, func(e hooks.HookEvent) {
		if c.OnClose != nil {
			c.OnClose()
		} else if c.Open != nil {
			c.Open.Set(false)
		}
	})

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+3)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, hookAttr)
	renderOpts = append(renderOpts, closeHandler)
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Div(renderOpts...)
}
