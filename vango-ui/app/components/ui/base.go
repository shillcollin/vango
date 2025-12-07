package ui

import "github.com/vango-dev/vango/v2/pkg/vdom"

// NodeOption is an alias for any, as Vango V2 uses variadic any for options
type NodeOption = any

// BaseConfig is embedded in every component config
type BaseConfig struct {
	Classes []string
	Options []NodeOption // Merged list of Attributes and Children
}

// ConfigProvider interface allows generic options to work on any config
type ConfigProvider interface {
	GetBase() *BaseConfig
}

// Option is a generic option function that modifies a ConfigProvider
type Option[T ConfigProvider] func(T)

// Class adds utility classes (merged via CN later)
func Class[T ConfigProvider](c string) Option[T] {
	return func(cfg T) {
		base := cfg.GetBase()
		base.Classes = append(base.Classes, c)
	}
}

// Attr allows passing raw Vango attributes (escape hatch)
func Attr[T ConfigProvider](attr NodeOption) Option[T] {
	return func(cfg T) {
		base := cfg.GetBase()
		base.Options = append(base.Options, attr)
	}
}

// Child allows passing children (strongly typed)
func Child[T ConfigProvider](nodes ...*vdom.VNode) Option[T] {
	return func(cfg T) {
		base := cfg.GetBase()
		for _, n := range nodes {
			base.Options = append(base.Options, n)
		}
	}
}
