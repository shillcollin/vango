// Package app contains the Vango UI components for the webdemo.
package app

import (
	"context"
	"fmt"

	"webdemo-vango/chat"

	"github.com/vango-dev/vango/v2/pkg/server"
	"github.com/vango-dev/vango/v2/pkg/vango"
	. "github.com/vango-dev/vango/v2/pkg/vdom"
)

// RootComponent is the main application component with persistent state.
type RootComponent struct {
	svc *chat.Service

	// State signals - created once, persist across renders
	providers    *vango.Signal[[]chat.ProviderInfo]
	messages     *vango.Signal[[]chat.Message]
	providerId   *vango.Signal[string]
	model        *vango.Signal[string]
	temperature  *vango.Signal[float64]
	theme        *vango.Signal[string]
	loading      *vango.Signal[bool]
	errorMsg     *vango.Signal[string]
	composerText *vango.Signal[string]
	testCounter  *vango.IntSignal // Simple test counter
}

// Root returns the root component for the application.
func Root(svc *chat.Service) server.Component {
	// Create component with persistent state
	c := &RootComponent{
		svc:          svc,
		providers:    vango.NewSignal(svc.Providers()),
		messages:     vango.NewSignal([]chat.Message{}),
		providerId:   vango.NewSignal(""),
		model:        vango.NewSignal(""),
		temperature:  vango.NewSignal(0.7),
		theme:        vango.NewSignal("dark"),
		loading:      vango.NewSignal(false),
		errorMsg:     vango.NewSignal(""),
		composerText: vango.NewSignal(""),
		testCounter:  vango.NewIntSignal(0),
	}

	// Initialize with first provider
	list := c.providers.Get()
	if len(list) > 0 {
		first := list[0]
		c.providerId.Set(first.ID)
		c.model.Set(first.DefaultModel)
	}

	return c
}

// Render implements server.Component
func (c *RootComponent) Render() *VNode {
	// Get active provider
	activeProvider := func() *chat.ProviderInfo {
		id := c.providerId.Get()
		for _, p := range c.providers.Get() {
			if p.ID == id {
				return &p
			}
		}
		return nil
	}

	// Event handlers
	handleProviderChange := func(id string) {
		c.providerId.Set(id)
		for _, p := range c.providers.Get() {
			if p.ID == id {
				c.model.Set(p.DefaultModel)
				break
			}
		}
	}

	handleSend := func() {
		text := c.composerText.Get()
		fmt.Printf("[DEBUG] handleSend called, text=%q\n", text)
		if text == "" {
			return
		}

		// Clear input
		c.composerText.Set("")

		// Create user message
		userMsg := chat.NewMessage("user", text)
		c.messages.Set(append(c.messages.Get(), userMsg))

		// Start streaming response
		go func() {
			c.loading.Set(true)
			defer c.loading.Set(false)

			// Create placeholder for assistant message
			assistantID := ""

			// Use a buffered channel so we don't block the service
			stream := make(chan chat.StreamEvent, 100)

			// Get current messages
			history := c.messages.Get()

			req := chat.ChatRequest{
				Provider:    c.providerId.Get(),
				Model:       c.model.Get(),
				Messages:    history,
				Temperature: float32(c.temperature.Get()),
			}

			// Launch request in background
			go func() {
				err := c.svc.SendMessage(context.Background(), req, stream)
				if err != nil {
					// We can't write to stream if SendMessage closed it?
					// SendMessage defers close(stream), so it will be closed.
					// We should probably just log or set error state if possible.
					fmt.Printf("[ERROR] SendMessage failed: %v\n", err)
					// We might want to send an error event if the channel isn't closed yet
					// but that's risky. Better to rely on the UI timeout or just the error log.
				}
			}()

			// Process stream events
			for event := range stream {
				if event.Type == "error" {
					c.errorMsg.Set("Stream error occurred") // Simplified
					break
				}

				if event.Type == "text.delta" && event.TextDelta != "" {
					if assistantID == "" {
						// First chunk - create assistant message
						assistantMsg := chat.NewMessage("assistant", event.TextDelta)
						assistantMsg.Provider = req.Provider
						assistantMsg.Model = req.Model
						assistantMsg.Status = "streaming"
						assistantID = assistantMsg.ID

						c.messages.Set(append(c.messages.Get(), assistantMsg))
					} else {
						// Append to existing message
						msgs := c.messages.Get()
						// Copy to trigger update
						updatedMsgs := make([]chat.Message, len(msgs))
						copy(updatedMsgs, msgs)

						for i, m := range updatedMsgs {
							if m.ID == assistantID {
								updatedMsgs[i].Content += event.TextDelta
								break
							}
						}
						c.messages.Set(updatedMsgs)
					}
				}

				if event.Type == "finish" {
					// Mark as complete
					msgs := c.messages.Get()
					updatedMsgs := make([]chat.Message, len(msgs))
					copy(updatedMsgs, msgs)
					for i, m := range updatedMsgs {
						if m.ID == assistantID {
							updatedMsgs[i].Status = "complete"
							break
						}
					}
					c.messages.Set(updatedMsgs)
				}
			}
		}()
	}

	handleClear := func() {
		c.messages.Set([]chat.Message{})
	}

	// Build providers list for select
	providerOptions := Range(c.providers.Get(), func(p chat.ProviderInfo, _ int) *VNode {
		attrs := []any{Value(p.ID), Text(p.Label)}
		if p.ID == c.providerId.Get() {
			attrs = append(attrs, Selected())
		}
		return Option(attrs...)
	})

	// Build models list
	var modelButtons []*VNode
	if ap := activeProvider(); ap != nil {
		modelButtons = Range(ap.Models, func(m string, _ int) *VNode {
			class := "pill"
			if m == c.model.Get() {
				class += " active"
			}
			return Button(
				Type("button"),
				Class(class),
				OnClick(func() { c.model.Set(m) }),
				Text(m),
			)
		})
	}

	// Build messages
	messageNodes := Range(c.messages.Get(), func(msg chat.Message, _ int) *VNode {
		class := "chat-message " + msg.Role
		return Article(Class(class), Key(msg.ID),
			If(msg.Role == "assistant" && msg.Provider != "",
				Header(Class("chat-message-header"),
					Span(Class("chat-model"), Text(msg.Provider+" • "+msg.Model)),
					If(msg.Status == "streaming",
						Span(Class("chat-status"), Text("Streaming…")),
					),
				),
			),
			If(msg.Content != "",
				P(Class("chat-text"), Text(msg.Content)),
			),
		)
	})

	return Div(Class("app-shell"), Data("theme", c.theme.Get()),
		// Sidebar
		Aside(Class("sidebar"),
			Div(Class("sidebar-header"),
				Div(Class("sidebar-brand"),
					Span(Class("sidebar-title"), Text("Vango Demo")),
				),
				P(Class("sidebar-subtitle"), Text("AI chat powered by Vango V2")),
			),

			// TEST: Simple counter to verify clicks work
			Section(Class("sidebar-section"),
				H3(Text("Test Counter & Send")),
				Div(Class("sidebar-pill-group"),
					Span(Textf("Count: %d", c.testCounter.Get())),
					Button(
						Type("button"),
						Class("pill"),
						OnClick(func() { c.testCounter.Inc() }),
						Text("+"),
					),
					Button(
						Type("button"),
						Class("pill"),
						OnClick(func() {
							fmt.Println("[DEBUG] Test Send clicked!")
							c.messages.Set(append(c.messages.Get(), chat.NewMessage("user", "Test message")))
						}),
						Text("Test Send"),
					),
				),
				Span(Textf("Messages: %d", len(c.messages.Get()))),
			),

			Section(Class("sidebar-section"),
				H3(Text("Provider")),
				Select(
					Class("provider-dropdown"),
					OnChange(handleProviderChange),
					providerOptions,
				),
			),

			Section(Class("sidebar-section"),
				H3(Text("Model")),
				Div(Class("sidebar-pill-group"),
					modelButtons,
				),
			),

			Section(Class("sidebar-spacer")),

			Section(Class("sidebar-section sidebar-footer"),
				Div(Class("sidebar-footer-controls"),
					Button(
						Type("button"),
						Class("ghost"),
						OnClick(handleClear),
						Text("Clear chat"),
					),
				),
			),
		),

		// Main content
		Main(Class("main"),
			// Error display
			If(c.errorMsg.Get() != "",
				Div(Class("alert"),
					Span(Text(c.errorMsg.Get())),
					Button(Type("button"), OnClick(func() { c.errorMsg.Set("") }), Text("Dismiss")),
				),
			),

			// Chat thread
			Div(Class("chat-thread"),
				If(len(c.messages.Get()) == 0,
					Div(Class("empty-state"),
						H2(Text("Start a conversation")),
						P(Text("Choose a provider on the left, then ask a question.")),
					),
				),
				messageNodes,
			),

			// Composer - use button click instead of form submit
			Div(Class("composer"),
				Textarea(
					Class("composer-input"),
					Placeholder("Ask anything..."),
					Value(c.composerText.Get()),
					OnInput(func(v string) {
						fmt.Printf("[DEBUG] OnInput called with: %q\n", v)
						c.composerText.Set(v)
					}),
				),
				Div(Class("composer-footer"),
					Div(Class("composer-submit"),
						Button(
							Type("button"),
							Class("primary"),
							OnClick(handleSend),
							Text("Send"),
						),
					),
				),
			),
		),
	)
}
