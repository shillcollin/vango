package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shillcollin/gai/core"
	"github.com/shillcollin/gai/obs"
	"github.com/shillcollin/gai/prompts"
	"github.com/shillcollin/gai/runner"
)

type chatHandler struct {
	providers map[string]providerEntry
	tavily    *tavilyClient
	prompt    promptInfo
	prompts   *prompts.Registry
	toolLimit promptTemplate
}

const maxConsecutiveToolSteps = 4

type apiMessage struct {
	Role     string         `json:"role"`
	Parts    []apiPart      `json:"parts"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type apiPart struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Data     string         `json:"data,omitempty"`
	Mime     string         `json:"mime,omitempty"`
	ID       string         `json:"id,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type chatRequest struct {
	Provider        string         `json:"provider"`
	Model           string         `json:"model,omitempty"`
	Mode            string         `json:"mode,omitempty"`
	Messages        []apiMessage   `json:"messages"`
	Temperature     float32        `json:"temperature,omitempty"`
	MaxOutputTokens int            `json:"max_output_tokens,omitempty"`
	ToolChoice      string         `json:"tool_choice,omitempty"`
	Tools           []string       `json:"tools,omitempty"`
	ProviderOptions map[string]any `json:"provider_options,omitempty"`
}

type chatResponse struct {
	ID           string          `json:"id"`
	Text         string          `json:"text"`
	JSON         any             `json:"json,omitempty"`
	Model        string          `json:"model"`
	Provider     string          `json:"provider"`
	Usage        core.Usage      `json:"usage"`
	FinishReason core.StopReason `json:"finish_reason"`
	Steps        []stepDTO       `json:"steps"`
	Warnings     []core.Warning  `json:"warnings,omitempty"`
}

type stepDTO struct {
	Number    int                `json:"number"`
	Text      string             `json:"text"`
	Model     string             `json:"model"`
	Duration  int64              `json:"duration_ms"`
	ToolCalls []toolExecutionDTO `json:"tool_calls"`
}

type toolExecutionDTO struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input,omitempty"`
	Result   any            `json:"result,omitempty"`
	Error    string         `json:"error,omitempty"`
	Duration int64          `json:"duration_ms,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Retries  int            `json:"retries,omitempty"`
}

type providerListResponse struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	DefaultModel string            `json:"default_model"`
	Models       []string          `json:"models"`
	Capabilities core.Capabilities `json:"capabilities"`
	Tools        []string          `json:"tools"`
	SystemPrompt string            `json:"system_prompt"`
	PromptMeta   map[string]string `json:"prompt_metadata,omitempty"`
}

func (h *chatHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	list := make([]providerListResponse, 0, len(h.providers))
	for id, entry := range h.providers {
		caps := entry.Client.Capabilities()
		tools := []string{}
		if h.tavily != nil && h.tavily.enabled() {
			tools = append(tools, "web_search", "url_extract")
		}
		promptMeta := h.promptMetadataStrings()
		list = append(list, providerListResponse{
			ID:           id,
			Label:        entry.Label,
			DefaultModel: entry.DefaultModel,
			Models:       entry.Models,
			Capabilities: caps,
			Tools:        tools,
			SystemPrompt: h.prompt.Text,
			PromptMeta:   promptMeta,
		})
	}
	writeJSON(w, http.StatusOK, list)
	log.Printf("providers request served (%d providers)", len(list))
}

func (h *chatHandler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid json: %v", err))
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages are required")
		return
	}

	ctx := r.Context()
	start := time.Now()
	reqID := uuid.NewString()

	requestCore, entry, err := h.prepareCoreRequest(req, reqID)
	if err != nil {
		if errors.Is(err, errUnknownProvider) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("chat request provider=%s mode=%s streaming=false", entry.Label, req.Mode)

	if strings.EqualFold(req.Mode, "json") {
		h.handleJSONMode(ctx, w, requestCore, entry, reqID, start)
		return
	}

	run := runner.New(entry.Client,
		runner.WithOnToolError(runner.ToolErrorAppendAndContinue),
		runner.WithToolTimeout(25*time.Second),
	)

	result, err := run.ExecuteRequest(ctx, requestCore)
	if err != nil {
		log.Printf("chat error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := chatResponse{
		ID:           uuid.NewString(),
		Text:         strings.TrimSpace(result.Text),
		Model:        result.Model,
		Provider:     entry.Client.Capabilities().Provider,
		Usage:        result.Usage,
		FinishReason: result.FinishReason,
		Warnings:     result.Warnings,
	}
	resp.Steps = convertSteps(result.Steps)

	latency := time.Since(start)
	metadata := map[string]any{"mode": "text", "streaming": false}
	if len(result.Warnings) > 0 {
		metadata["warnings"] = result.Warnings
	}

	obs.LogCompletion(ctx, obs.Completion{
		Provider:     resp.Provider,
		Model:        result.Model,
		RequestID:    reqID,
		Input:        obs.MessagesFromCore(requestCore.Messages),
		Output:       obs.MessageFromCore(core.AssistantMessage(resp.Text)),
		Usage:        obs.UsageFromCore(result.Usage),
		LatencyMS:    latency.Milliseconds(),
		Metadata:     metadata,
		ToolCalls:    obs.ToolCallsFromSteps(result.Steps),
		CreatedAtUTC: time.Now().UTC().UnixMilli(),
	})

	writeJSON(w, http.StatusOK, resp)
	log.Printf("chat response sent (latency=%s)", latency)
}

func (h *chatHandler) handleJSONMode(ctx context.Context, w http.ResponseWriter, request core.Request, entry providerEntry, reqID string, start time.Time) {
	result, err := entry.Client.GenerateObject(ctx, request)
	if err != nil {
		log.Printf("chat error (json mode): %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var payload any
	if len(result.JSON) > 0 {
		if err := json.Unmarshal(result.JSON, &payload); err != nil {
			payload = string(result.JSON)
		}
	}

	resp := chatResponse{
		ID:           uuid.NewString(),
		JSON:         payload,
		Model:        result.Model,
		Provider:     entry.Client.Capabilities().Provider,
		Usage:        result.Usage,
		FinishReason: core.StopReason{Type: core.StopReasonProviderFinish},
	}

	latency := time.Since(start)
	obs.LogCompletion(ctx, obs.Completion{
		Provider:     resp.Provider,
		Model:        result.Model,
		RequestID:    reqID,
		Input:        obs.MessagesFromCore(request.Messages),
		Output:       obs.MessageFromCore(core.AssistantMessage(string(result.JSON))),
		Usage:        obs.UsageFromCore(result.Usage),
		LatencyMS:    latency.Milliseconds(),
		Metadata:     map[string]any{"mode": "json", "streaming": false},
		CreatedAtUTC: time.Now().UTC().UnixMilli(),
	})

	writeJSON(w, http.StatusOK, resp)
	log.Printf("chat response sent (json mode, latency=%s)", latency)
}

func (h *chatHandler) prepareCoreRequest(req chatRequest, reqID string) (core.Request, providerEntry, error) {
	entry, ok := h.providers[req.Provider]
	if !ok {
		return core.Request{}, providerEntry{}, errUnknownProvider
	}

	messages, err := convertAPIMessages(req.Messages)
	if err != nil {
		return core.Request{}, providerEntry{}, err
	}
	messages = h.ensureSystemMessage(messages)

	selected := make(map[string]struct{}, len(req.Tools))
	for _, name := range req.Tools {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			selected[trimmed] = struct{}{}
		}
	}

	toolHandles := make([]core.ToolHandle, 0, 4)
	if h.tavily != nil && h.tavily.enabled() {
		if len(selected) == 0 || hasTool(selected, "web_search") {
			if handle := h.tavily.searchTool(); handle != nil {
				toolHandles = append(toolHandles, handle)
			}
		}
		if len(selected) == 0 || hasTool(selected, "url_extract") {
			if handle := h.tavily.extractTool(); handle != nil {
				toolHandles = append(toolHandles, handle)
			}
		}
	}

	providerOptions := cloneAnyMap(req.ProviderOptions)
	if providerOptions == nil {
		providerOptions = map[string]any{}
	}

	metadata := map[string]any{"request_id": reqID}
	if promptMeta := h.promptMetadataStrings(); len(promptMeta) > 0 {
		for k, v := range promptMeta {
			metadata[k] = v
		}
	}

	request := core.Request{
		Model:           chooseModel(req.Model, entry.DefaultModel),
		Messages:        messages,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxOutputTokens,
		Tools:           toolHandles,
		ToolChoice:      parseToolChoice(req.ToolChoice),
		ProviderOptions: providerOptions,
		Metadata:        metadata,
	}

	request.StopWhen = core.Any(
		stopWhenMaxConsecutiveToolSteps(maxConsecutiveToolSteps),
		core.NoMoreTools(),
	)
	request.OnStop = h.buildConsecutiveToolFinalizer(entry, request, maxConsecutiveToolSteps)

	return request, entry, nil
}

func convertSteps(steps []core.Step) []stepDTO {
	if len(steps) == 0 {
		return nil
	}
	out := make([]stepDTO, 0, len(steps))
	for _, step := range steps {
		dto := stepDTO{
			Number:   step.Number,
			Text:     strings.TrimSpace(step.Text),
			Model:    step.Model,
			Duration: step.DurationMS,
		}
		if len(step.ToolCalls) > 0 {
			dto.ToolCalls = make([]toolExecutionDTO, 0, len(step.ToolCalls))
			for _, exec := range step.ToolCalls {
				dto.ToolCalls = append(dto.ToolCalls, toolExecutionDTO{
					ID:       exec.Call.ID,
					Name:     exec.Call.Name,
					Input:    obs.NormalizeMap(exec.Call.Input),
					Result:   normalizeAny(exec.Result),
					Error:    errorToString(exec.Error),
					Duration: exec.DurationMS,
					Metadata: obs.NormalizeMap(exec.Call.Metadata),
					Retries:  exec.Retries,
				})
			}
		}
		out = append(out, dto)
	}
	return out
}

func convertAPIMessages(msgs []apiMessage) ([]core.Message, error) {
	converted := make([]core.Message, 0, len(msgs))
	for _, msg := range msgs {
		role := core.Role(strings.ToLower(strings.TrimSpace(msg.Role)))
		switch role {
		case core.System, core.User, core.Assistant:
		default:
			return nil, fmt.Errorf("unsupported role %s", msg.Role)
		}
		parts := make([]core.Part, 0, len(msg.Parts))
		for _, part := range msg.Parts {
			switch strings.ToLower(part.Type) {
			case "text":
				parts = append(parts, core.Text{Text: part.Text})
			case "image_base64":
				data, err := base64.StdEncoding.DecodeString(part.Data)
				if err != nil {
					return nil, fmt.Errorf("invalid image data: %w", err)
				}
				mime := part.Mime
				if mime == "" {
					mime = "image/png"
				}
				parts = append(parts, core.Image{Source: core.BlobRef{Kind: core.BlobBytes, Bytes: data, MIME: mime, Size: int64(len(data))}})
			case "image_url":
				if strings.TrimSpace(part.Data) == "" {
					return nil, errors.New("image_url part requires data")
				}
				parts = append(parts, core.ImageURL{URL: part.Data, MIME: part.Mime})
			case "function_call":
				args := map[string]any{}
				if part.Text != "" {
					if err := json.Unmarshal([]byte(part.Text), &args); err != nil {
						return nil, fmt.Errorf("invalid function call args: %w", err)
					}
				}
				parts = append(parts, core.ToolCall{
					ID:       part.ID,
					Name:     part.Mime,
					Input:    args,
					Metadata: cloneMap(part.Metadata),
				})
			case "function_response":
				var response map[string]any
				if part.Text != "" {
					if err := json.Unmarshal([]byte(part.Text), &response); err != nil {
						return nil, fmt.Errorf("invalid function response: %w", err)
					}
				}
				if response == nil {
					response = map[string]any{}
				}
				parts = append(parts, core.ToolResult{ID: part.ID, Name: part.Mime, Result: response})
			default:
				return nil, fmt.Errorf("unsupported part type %s", part.Type)
			}
		}
		converted = append(converted, core.Message{Role: role, Parts: parts, Metadata: cloneMap(msg.Metadata)})
	}
	return converted, nil
}

func chooseModel(requested, fallback string) string {
	if strings.TrimSpace(requested) != "" {
		return requested
	}
	return fallback
}

func parseToolChoice(choice string) core.ToolChoice {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "none":
		return core.ToolChoiceNone
	case "required":
		return core.ToolChoiceRequired
	default:
		return core.ToolChoiceAuto
	}
}

func hasTool(set map[string]struct{}, name string) bool {
	_, ok := set[name]
	return ok
}

func (h *chatHandler) promptMetadataStrings() map[string]string {
	if strings.TrimSpace(h.prompt.Text) == "" {
		return nil
	}
	meta := map[string]string{}
	if h.prompt.Name != "" {
		meta["prompt_name"] = h.prompt.Name
	}
	if h.prompt.Version != "" {
		meta["prompt_version"] = h.prompt.Version
	}
	if h.prompt.Fingerprint != "" {
		meta["prompt_fingerprint"] = h.prompt.Fingerprint
	}
	return meta
}

func (h *chatHandler) ensureSystemMessage(msgs []core.Message) []core.Message {
	promptText := strings.TrimSpace(h.prompt.Text)
	if promptText == "" {
		return msgs
	}
	metaStrings := h.promptMetadataStrings()
	systemMsg := core.SystemMessage(promptText)
	if len(metaStrings) > 0 {
		systemMsg.Metadata = make(map[string]any, len(metaStrings))
		for k, v := range metaStrings {
			systemMsg.Metadata[k] = v
		}
	}
	if len(msgs) == 0 {
		return append([]core.Message{systemMsg}, msgs...)
	}
	first := msgs[0]
	if first.Role != core.System {
		return append([]core.Message{systemMsg}, msgs...)
	}
	if len(first.Parts) == 1 {
		if textPart, ok := first.Parts[0].(core.Text); ok && strings.TrimSpace(textPart.Text) == promptText {
			if len(metaStrings) > 0 {
				if first.Metadata == nil {
					first.Metadata = make(map[string]any, len(metaStrings))
				}
				for k, v := range metaStrings {
					first.Metadata[k] = v
				}
			}
			msgs[0] = first
			return msgs
		}
	}
	msgs[0] = systemMsg
	return msgs
}

func (h *chatHandler) renderToolLimitInstruction(ctx context.Context, limit int) (string, promptTemplate, error) {
	if h.prompts == nil || h.toolLimit.Name == "" {
		return "", promptTemplate{}, errors.New("tool limit prompt unavailable")
	}
	data := map[string]any{"Limit": limit}
	text, id, err := h.prompts.Render(ctx, h.toolLimit.Name, h.toolLimit.Version, data)
	if err != nil {
		return "", promptTemplate{}, err
	}
	info := promptTemplate{Name: id.Name, Version: id.Version, Fingerprint: id.Fingerprint}
	return text, info, nil
}

func stopWhenMaxConsecutiveToolSteps(limit int) core.StopCondition {
	if limit <= 0 {
		limit = 1
	}
	return func(state *core.RunnerState) (bool, core.StopReason) {
		if state == nil {
			return false, core.StopReason{}
		}
		consecutive := 0
		for i := len(state.Steps) - 1; i >= 0; i-- {
			if len(state.Steps[i].ToolCalls) == 0 {
				break
			}
			consecutive++
		}
		if consecutive >= limit {
			return true, core.StopReason{
				Type:        core.StopReasonMaxSteps,
				Description: fmt.Sprintf("reached %d consecutive tool-call steps", limit),
				Details: map[string]any{
					"limit":       limit,
					"consecutive": consecutive,
				},
			}
		}
		return false, core.StopReason{}
	}
}

func (h *chatHandler) buildConsecutiveToolFinalizer(entry providerEntry, base core.Request, limit int) core.Finalizer {
	if entry.Client == nil {
		return nil
	}
	providerName := entry.Client.Capabilities().Provider
	baseMetadata := cloneAnyMap(base.Metadata)
	baseProviderOptions := cloneAnyMap(base.ProviderOptions)
	baseModel := base.Model

	return func(ctx context.Context, state core.FinalState) (*core.TextResult, error) {
		baseResult := stateToResult(providerName, baseModel, state)
		if state.StopReason.Type != core.StopReasonMaxSteps {
			return baseResult, nil
		}

		messages := append([]core.Message(nil), state.Messages...)
		instructionText, instructionPrompt, promptErr := h.renderToolLimitInstruction(ctx, limit)
		if promptErr != nil || strings.TrimSpace(instructionText) == "" {
			if promptErr != nil {
				log.Printf("tool limit prompt render error: %v", promptErr)
			}
			instructionText = fmt.Sprintf(
				"The tool runner stopped after %d consecutive tool-call steps. Using only the information already gathered, craft a final reply that explains the limit, summarizes findings, and suggests concrete next steps. Do not call any tools. You should see the content results from the past %d tool calls; summarize them and note any missing data. Lead with TOOL LIMIT HIT.",
				limit, limit,
			)
			instructionPrompt = promptTemplate{}
		}
		systemInstruction := core.SystemMessage(instructionText)
		if instructionPrompt.Name != "" || instructionPrompt.Version != "" || instructionPrompt.Fingerprint != "" {
			meta := map[string]any{}
			if instructionPrompt.Name != "" {
				meta["prompt_name"] = instructionPrompt.Name
			}
			if instructionPrompt.Version != "" {
				meta["prompt_version"] = instructionPrompt.Version
			}
			if instructionPrompt.Fingerprint != "" {
				meta["prompt_fingerprint"] = instructionPrompt.Fingerprint
			}
			systemInstruction.Metadata = meta
		}
		messages = append(messages,
			systemInstruction,
			core.UserMessage(core.TextPart("Respond to the user now with a complete answer.")),
		)

		metadata := cloneAnyMap(baseMetadata)
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["finalizer"] = "consecutive_tool_limit"
		metadata["finalizer_limit"] = fmt.Sprintf("%d", limit)
		if instructionPrompt.Name != "" {
			metadata["finalizer_prompt_name"] = instructionPrompt.Name
		}
		if instructionPrompt.Version != "" {
			metadata["finalizer_prompt_version"] = instructionPrompt.Version
		}
		if instructionPrompt.Fingerprint != "" {
			metadata["finalizer_prompt_fingerprint"] = instructionPrompt.Fingerprint
		}

		providerOptions := cloneAnyMap(baseProviderOptions)

		finalReq := core.Request{
			Model:           firstNonEmpty(baseModel, lastModelFromSteps(state.Steps, baseModel)),
			Messages:        messages,
			Temperature:     base.Temperature,
			MaxTokens:       base.MaxTokens,
			TopP:            base.TopP,
			TopK:            base.TopK,
			ToolChoice:      core.ToolChoiceNone,
			Tools:           nil,
			Metadata:        metadata,
			ProviderOptions: providerOptions,
		}

		result, err := entry.Client.GenerateText(ctx, finalReq)
		if err != nil {
			fallback := strings.TrimSpace(baseResult.Text)
			var message string
			if fallback == "" {
				message = fmt.Sprintf("I reached the limit of %d consecutive tool calls and could not complete the request. Please retry or adjust the instructions.", limit)
			} else {
				message = fmt.Sprintf("I reached the limit of %d consecutive tool calls before finishing. Here's the partial answer I produced:\n\n%s", limit, fallback)
			}
			log.Printf("OnStop finalizer GenerateText error: %v", err)
			baseResult.Text = message
			now := time.Now()
			baseResult.Steps = append(baseResult.Steps, core.Step{
				Number:      len(baseResult.Steps) + 1,
				Text:        message,
				Usage:       core.Usage{},
				DurationMS:  0,
				StartedAt:   now.UnixMilli(),
				CompletedAt: now.UnixMilli(),
				Model:       firstNonEmpty(baseModel, lastModelFromSteps(baseResult.Steps, baseModel), providerName),
			})
			return baseResult, nil
		}

		finishReason := cloneStopReason(state.StopReason)
		if finishReason.Details == nil {
			finishReason.Details = map[string]any{}
		}
		finishReason.Details["finalizer"] = "consecutive_tool_limit"
		finishReason.Details["limit"] = limit

		combinedUsage := combineUsage(state.Usage, result.Usage)

		finalText := strings.TrimSpace(result.Text)
		if finalText == "" {
			finalText = baseResult.Text
		}

		steps := append([]core.Step(nil), state.Steps...)
		if len(result.Steps) > 0 {
			steps = append(steps, result.Steps...)
		} else if finalText != "" {
			duplicate := len(steps) > 0 && strings.TrimSpace(steps[len(steps)-1].Text) == finalText
			if !duplicate {
				now := time.Now()
				steps = append(steps, core.Step{
					Number:      len(steps) + 1,
					Text:        finalText,
					Usage:       result.Usage,
					DurationMS:  0,
					StartedAt:   now.UnixMilli(),
					CompletedAt: now.UnixMilli(),
					Model:       firstNonEmpty(result.Model, lastModelFromSteps(steps, baseModel), baseModel),
				})
			}
		}

		warnings := append([]core.Warning(nil), baseResult.Warnings...)
		if len(result.Warnings) > 0 {
			warnings = append(warnings, result.Warnings...)
		}

		finalModel := firstNonEmpty(result.Model, lastModelFromSteps(steps, baseModel), baseModel)
		finalProvider := firstNonEmpty(result.Provider, providerName)

		return &core.TextResult{
			Text:         finalText,
			Steps:        steps,
			Usage:        combinedUsage,
			FinishReason: finishReason,
			Provider:     finalProvider,
			Model:        finalModel,
			Warnings:     warnings,
		}, nil
	}
}

func stateToResult(providerName, fallbackModel string, state core.FinalState) *core.TextResult {
	text := ""
	if state.LastText != nil {
		text = strings.TrimSpace(state.LastText())
	}
	steps := append([]core.Step(nil), state.Steps...)
	model := lastModelFromSteps(steps, fallbackModel)
	return &core.TextResult{
		Text:         text,
		Steps:        steps,
		Usage:        state.Usage,
		FinishReason: state.StopReason,
		Provider:     providerName,
		Model:        model,
	}
}

func combineUsage(a, b core.Usage) core.Usage {
	return core.Usage{
		InputTokens:       a.InputTokens + b.InputTokens,
		OutputTokens:      a.OutputTokens + b.OutputTokens,
		ReasoningTokens:   a.ReasoningTokens + b.ReasoningTokens,
		TotalTokens:       a.TotalTokens + b.TotalTokens,
		CostUSD:           a.CostUSD + b.CostUSD,
		CachedInputTokens: a.CachedInputTokens + b.CachedInputTokens,
		AudioTokens:       a.AudioTokens + b.AudioTokens,
	}
}

func cloneStopReason(reason core.StopReason) core.StopReason {
	cloned := reason
	if len(reason.Details) > 0 {
		cloned.Details = make(map[string]any, len(reason.Details))
		for k, v := range reason.Details {
			cloned.Details[k] = v
		}
	}
	return cloned
}

func lastModelFromSteps(steps []core.Step, fallback string) string {
	for i := len(steps) - 1; i >= 0; i-- {
		if model := strings.TrimSpace(steps[i].Model); model != "" {
			return model
		}
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeAny(v any) any {
	if v == nil {
		return nil
	}
	switch typed := v.(type) {
	case string, float64, float32, int, int64, uint64, bool, map[string]any, []any:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			return string(data)
		}
		return generic
	}
}

func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": message})
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneAnyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

var errUnknownProvider = errors.New("unknown provider")
