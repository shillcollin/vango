package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/vdom"
)

// 1. Define Typed Enums
type ButtonVariant string

const (
	ButtonVariantDefault     ButtonVariant = "default"
	ButtonVariantPrimary     ButtonVariant = "primary"
	ButtonVariantDestructive ButtonVariant = "destructive"
	ButtonVariantOutline     ButtonVariant = "outline"
	ButtonVariantSecondary   ButtonVariant = "secondary"
	ButtonVariantGhost       ButtonVariant = "ghost"
	ButtonVariantLink        ButtonVariant = "link"
)

type ButtonSize string

const (
	ButtonSizeDefault ButtonSize = "default"
	ButtonSizeSm      ButtonSize = "sm"
	ButtonSizeLg      ButtonSize = "lg"
	ButtonSizeIcon    ButtonSize = "icon"
)

// 2. Define Component Config
type ButtonConfig struct {
	BaseConfig // Embeds Classes, Options
	Variant    ButtonVariant
	Size       ButtonSize
}

// Implement ConfigProvider interface
func (c *ButtonConfig) GetBase() *BaseConfig { return &c.BaseConfig }

// 3. Define Option Type Alias (for better DX)
type ButtonOption = Option[*ButtonConfig]

// 4. Define Component-Specific Options
func Variant(v ButtonVariant) ButtonOption {
	return func(c *ButtonConfig) { c.Variant = v }
}

func Size(s ButtonSize) ButtonOption {
	return func(c *ButtonConfig) { c.Size = s }
}

// 5. Implementation
func Button(opts ...ButtonOption) *vdom.VNode {
	// Default config
	c := &ButtonConfig{
		Variant: ButtonVariantDefault,
		Size:    ButtonSizeDefault,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Resolve classes (Base classes + Variant classes + User overrides)
	variantClass := buttonVariants(c.Variant, c.Size)

	finalClass := CN(
		"inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium ring-offset-background transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50",
		variantClass,
		strings.Join(c.BaseConfig.Classes, " "),
	)

	// Render
	// Merge calculated class with user-provided options (attrs/children)
	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Button(renderOpts...)
}

func buttonVariants(v ButtonVariant, s ButtonSize) string {
	var classes []string

	switch v {
	case ButtonVariantDefault, ButtonVariantPrimary:
		classes = append(classes, "bg-primary text-primary-foreground hover:bg-primary/90")
	case ButtonVariantDestructive:
		classes = append(classes, "bg-destructive text-destructive-foreground hover:bg-destructive/90")
	case ButtonVariantOutline:
		classes = append(classes, "border border-input bg-background hover:bg-accent hover:text-accent-foreground")
	case ButtonVariantSecondary:
		classes = append(classes, "bg-secondary text-secondary-foreground hover:bg-secondary/80")
	case ButtonVariantGhost:
		classes = append(classes, "hover:bg-accent hover:text-accent-foreground")
	case ButtonVariantLink:
		classes = append(classes, "text-primary underline-offset-4 hover:underline")
	}

	switch s {
	case ButtonSizeDefault:
		classes = append(classes, "h-10 px-4 py-2")
	case ButtonSizeSm:
		classes = append(classes, "h-9 rounded-md px-3")
	case ButtonSizeLg:
		classes = append(classes, "h-11 rounded-md px-8")
	case ButtonSizeIcon:
		classes = append(classes, "h-10 w-10")
	}

	return strings.Join(classes, " ")
}
