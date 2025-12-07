package ui

import (
	"strings"

	"github.com/vango-dev/vango/v2/pkg/vdom"
)

type LabelConfig struct {
	BaseConfig
	For string
}

func (c *LabelConfig) GetBase() *BaseConfig { return &c.BaseConfig }

type LabelOption = Option[*LabelConfig]

func LabelFor(id string) LabelOption {
	return func(c *LabelConfig) { c.For = id }
}

func Label(opts ...LabelOption) *vdom.VNode {
	c := &LabelConfig{}
	for _, opt := range opts {
		opt(c)
	}

	finalClass := CN(
		"text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70",
		strings.Join(c.BaseConfig.Classes, " "),
	)

	renderOpts := make([]any, 0, len(c.BaseConfig.Options)+2)
	renderOpts = append(renderOpts, vdom.Class(finalClass))
	if c.For != "" {
		renderOpts = append(renderOpts, vdom.For(c.For))
	}
	renderOpts = append(renderOpts, c.BaseConfig.Options...)

	return vdom.Label(renderOpts...)
}
