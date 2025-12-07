package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/vdom"
)

// Card
type CardConfig struct{ BaseConfig }

func (c *CardConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardOption = Option[*CardConfig]

func Card(opts ...CardOption) *vdom.VNode {
	c := &CardConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"rounded-lg border bg-card text-card-foreground shadow-sm",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.Div(renderOpts...)
}

// CardHeader
type CardHeaderConfig struct{ BaseConfig }

func (c *CardHeaderConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardHeaderOption = Option[*CardHeaderConfig]

func CardHeader(opts ...CardHeaderOption) *vdom.VNode {
	c := &CardHeaderConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"flex flex-col space-y-1.5 p-6",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.Div(renderOpts...)
}

// CardTitle
type CardTitleConfig struct{ BaseConfig }

func (c *CardTitleConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardTitleOption = Option[*CardTitleConfig]

func CardTitle(opts ...CardTitleOption) *vdom.VNode {
	c := &CardTitleConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"text-2xl font-semibold leading-none tracking-tight",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.H3(renderOpts...)
}

// CardDescription
type CardDescriptionConfig struct{ BaseConfig }

func (c *CardDescriptionConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardDescriptionOption = Option[*CardDescriptionConfig]

func CardDescription(opts ...CardDescriptionOption) *vdom.VNode {
	c := &CardDescriptionConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"text-sm text-muted-foreground",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.P(renderOpts...)
}

// CardContent
type CardContentConfig struct{ BaseConfig }

func (c *CardContentConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardContentOption = Option[*CardContentConfig]

func CardContent(opts ...CardContentOption) *vdom.VNode {
	c := &CardContentConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"p-6 pt-0",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.Div(renderOpts...)
}

// CardFooter
type CardFooterConfig struct{ BaseConfig }

func (c *CardFooterConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type CardFooterOption = Option[*CardFooterConfig]

func CardFooter(opts ...CardFooterOption) *vdom.VNode {
	c := &CardFooterConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"flex items-center p-6 pt-0",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+1)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	renderOpts = append(renderOpts, c.BaseConfig.Options...)
	return vdom.Div(renderOpts...)
}
