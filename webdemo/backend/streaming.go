package main

import (
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
	"github.com/shillcollin/gai/runner"
)

type streamEventPayload struct {
	Type             string            `json:"type"`
	Step             int               `json:"step,omitempty"`
	Seq              int               `json:"seq"`
	TimestampMS      int64             `json:"ts"`
	Provider         string            `json:"provider,omitempty"`
	Model            string            `json:"model,omitempty"`
	TextDelta        string            `json:"text_delta,omitempty"`
	ReasoningDelta   string            `json:"reasoning_delta,omitempty"`
	ReasoningSummary string            `json:"reasoning_summary,omitempty"`
	ToolCall         *toolExecutionDTO `json:"tool_call,omitempty"`
	ToolResult       *toolExecutionDTO `json:"tool_result,omitempty"`
	Usage            *core.Usage       `json:"usage,omitempty"`
	FinishReason     *core.StopReason  `json:"finish_reason,omitempty"`
	Warnings         []core.Warning    `json:"warnings,omitempty"`
	Ext              map[string]any    `json:"ext,omitempty"`
}

func (h *chatHandler) handleChatStream(w http.ResponseWriter, r *http.Request) {
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

	if strings.EqualFold(req.Mode, "json") {
		writeError(w, http.StatusBadRequest, "streaming structured responses is not supported; use /api/chat")
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

	log.Printf("chat request provider=%s mode=%s streaming=true", entry.Label, req.Mode)

	run := runner.New(entry.Client,
		runner.WithOnToolError(runner.ToolErrorAppendAndContinue),
		runner.WithToolTimeout(25*time.Second),
	)

	stream, err := run.StreamRequest(ctx, requestCore)
	if err != nil {
		log.Printf("chat stream init error: %v", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer stream.Close()

	flusher, ok := w.(http.Flusher)
	if !ok {
		stream.Fail(errors.New("streaming not supported by server"))
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")

	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	providerName := entry.Client.Capabilities().Provider
	accumulator := newStreamAccumulator(start)

	for event := range stream.Events() {
		accumulator.consume(event)
		payload := makeStreamPayload(event, providerName)
		if err := encoder.Encode(payload); err != nil {
			stream.Fail(err)
			log.Printf("chat stream write error: %v", err)
			return
		}
		flusher.Flush()
	}

	if err := stream.Err(); err != nil && !errors.Is(err, core.ErrStreamClosed) {
		log.Printf("chat stream error: %v", err)
		return
	}

	meta := stream.Meta()
	accumulator.finalize(meta, stream.Warnings())

	usage := accumulator.usage
	if usage == (core.Usage{}) {
		usage = meta.Usage
	}
	model := meta.Model
	if model == "" {
		model = requestCore.Model
	}
	provider := meta.Provider
	if provider == "" {
		provider = providerName
	}

	latency := time.Since(start)
	metadata := map[string]any{"mode": "text", "streaming": true}
	if len(accumulator.warnings) > 0 {
		metadata["warnings"] = accumulator.warnings
	}
	if accumulator.err != nil {
		metadata["error"] = accumulator.err.Error()
	}

	obs.LogCompletion(ctx, obs.Completion{
		Provider:     provider,
		Model:        model,
		RequestID:    reqID,
		Input:        obs.MessagesFromCore(requestCore.Messages),
		Output:       obs.MessageFromCore(core.AssistantMessage(accumulator.text())),
		Usage:        obs.UsageFromCore(usage),
		LatencyMS:    latency.Milliseconds(),
		Metadata:     metadata,
		ToolCalls:    accumulator.records(),
		Error:        errorToString(accumulator.err),
		CreatedAtUTC: time.Now().UTC().UnixMilli(),
	})

	log.Printf("chat stream completed (latency=%s)", latency)
}

type streamAccumulator struct {
	started     time.Time
	textBuilder strings.Builder
	usage       core.Usage
	finish      *core.StopReason
	toolRecords map[string]*obs.ToolCallRecord
	order       []string
	warnings    []core.Warning
	err         error
}

func newStreamAccumulator(start time.Time) *streamAccumulator {
	return &streamAccumulator{
		started:     start,
		toolRecords: make(map[string]*obs.ToolCallRecord),
		order:       make([]string, 0),
	}
}

func (a *streamAccumulator) consume(event core.StreamEvent) {
    switch event.Type {
    case core.EventTextDelta:
        a.textBuilder.WriteString(event.TextDelta)
    case core.EventToolCall:
        // Use a composite key of step + id to avoid collisions
        // when providers reuse simple IDs like "call_1" across steps.
        id := event.ToolCall.ID
        key := id
        if key == "" {
            key = fmt.Sprintf("step%d-call%d", event.StepID, len(a.order)+1)
        } else {
            key = fmt.Sprintf("%d:%s", event.StepID, key)
        }
        record := &obs.ToolCallRecord{
            Step:  event.StepID,
            ID:    event.ToolCall.ID,
            Name:  event.ToolCall.Name,
            Input: obs.NormalizeMap(event.ToolCall.Input),
        }
		if len(event.ToolCall.Metadata) > 0 {
			meta := obs.NormalizeMap(event.ToolCall.Metadata)
			if len(meta) > 0 {
				if record.Input == nil {
					record.Input = map[string]any{}
				}
				record.Input["metadata"] = meta
			}
		}
        a.toolRecords[key] = record
        a.order = append(a.order, key)
    case core.EventToolResult:
        id := event.ToolResult.ID
        key := id
        if key == "" {
            key = fmt.Sprintf("step%d-result%d", event.StepID, len(a.order)+1)
        } else {
            key = fmt.Sprintf("%d:%s", event.StepID, key)
        }
        record, ok := a.toolRecords[key]
        if !ok {
            record = &obs.ToolCallRecord{Step: event.StepID, ID: event.ToolResult.ID, Name: event.ToolResult.Name}
            a.toolRecords[key] = record
            a.order = append(a.order, key)
        }
        record.Result = normalizeAny(event.ToolResult.Result)
        record.Error = event.ToolResult.Error
        if dur := extractInt64(event.Ext, "duration_ms"); dur > 0 {
            record.DurationMS = dur
        }
		if retries := extractInt(event.Ext, "retries"); retries > 0 {
			record.Retries = retries
		}
		if metadata := extractMetadata(event.Ext); len(metadata) > 0 {
			if record.Input == nil {
				record.Input = map[string]any{}
			}
			record.Input["metadata"] = metadata
		}
	case core.EventFinish:
		a.usage = event.Usage
		if event.FinishReason != nil {
			reason := *event.FinishReason
			a.finish = &reason
		}
	case core.EventError:
		if event.Error != nil {
			a.err = event.Error
		}
	}
}

func (a *streamAccumulator) finalize(meta core.StreamMeta, warnings []core.Warning) {
	if a.usage == (core.Usage{}) {
		a.usage = meta.Usage
	}
	if len(meta.Warnings) > 0 {
		a.warnings = append(a.warnings, meta.Warnings...)
	}
	if len(warnings) > 0 {
		a.warnings = append(a.warnings, warnings...)
	}
}

func (a *streamAccumulator) text() string {
	return strings.TrimSpace(a.textBuilder.String())
}

func (a *streamAccumulator) records() []obs.ToolCallRecord {
	if len(a.order) == 0 {
		return nil
	}
	out := make([]obs.ToolCallRecord, 0, len(a.order))
	for _, id := range a.order {
		if rec := a.toolRecords[id]; rec != nil {
			clone := *rec
			if len(rec.Input) > 0 {
				clone.Input = obs.NormalizeMap(rec.Input)
			}
			out = append(out, clone)
		}
	}
	return out
}

func makeStreamPayload(event core.StreamEvent, provider string) streamEventPayload {
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	payload := streamEventPayload{
		Type:        string(event.Type),
		Step:        event.StepID,
		Seq:         event.Seq,
		TimestampMS: ts.UTC().UnixMilli(),
		Provider:    provider,
		Model:       event.Model,
	}
	switch event.Type {
	case core.EventTextDelta:
		payload.TextDelta = event.TextDelta
	case core.EventReasoningDelta:
		payload.ReasoningDelta = event.ReasoningDelta
	case core.EventReasoningSummary:
		payload.ReasoningSummary = event.ReasoningSummary
    case core.EventToolCall:
        // Publish a step-scoped ID to avoid UI merges across steps.
        // Keep original provider ID in metadata when present.
        composedID := event.ToolCall.ID
        if composedID != "" {
            composedID = fmt.Sprintf("%d:%s", event.StepID, composedID)
        } else {
            composedID = fmt.Sprintf("%d:%d", event.StepID, event.Seq)
        }
        payload.ToolCall = &toolExecutionDTO{
            ID:       composedID,
            Name:     event.ToolCall.Name,
            Input:    obs.NormalizeMap(event.ToolCall.Input),
            Metadata: obs.NormalizeMap(event.ToolCall.Metadata),
        }
    case core.EventToolResult:
        // Mirror the step-scoped ID for pairing with the tool.call event.
        composedID := event.ToolResult.ID
        if composedID != "" {
            composedID = fmt.Sprintf("%d:%s", event.StepID, composedID)
        } else {
            composedID = fmt.Sprintf("%d:%d", event.StepID, event.Seq)
        }
        dto := &toolExecutionDTO{
            ID:       composedID,
            Name:     event.ToolResult.Name,
            Result:   normalizeAny(event.ToolResult.Result),
            Error:    event.ToolResult.Error,
            Metadata: extractMetadata(event.Ext),
        }
		if dur := extractInt64(event.Ext, "duration_ms"); dur > 0 {
			dto.Duration = dur
		}
		if retries := extractInt(event.Ext, "retries"); retries > 0 {
			dto.Retries = retries
		}
		payload.ToolResult = dto
	case core.EventFinish:
		usage := event.Usage
		payload.Usage = &usage
		if event.FinishReason != nil {
			reason := *event.FinishReason
			payload.FinishReason = &reason
		}
	case core.EventError:
		if event.Error != nil {
			payload.Ext = map[string]any{"error": event.Error.Error()}
		}
	}
	if len(event.Ext) > 0 {
		if payload.Ext == nil {
			payload.Ext = map[string]any{}
		}
		for k, v := range event.Ext {
			payload.Ext[k] = v
		}
	}
	return payload
}

func extractMetadata(ext map[string]any) map[string]any {
	if len(ext) == 0 {
		return nil
	}
	raw, ok := ext["metadata"]
	if !ok {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]any:
		return obs.NormalizeMap(typed)
	default:
		return map[string]any{"value": normalizeAny(raw)}
	}
}

func extractInt64(ext map[string]any, key string) int64 {
	if len(ext) == 0 {
		return 0
	}
	v, ok := ext[key]
	if !ok {
		return 0
	}
	switch typed := v.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

func extractInt(ext map[string]any, key string) int {
	if len(ext) == 0 {
		return 0
	}
	v, ok := ext[key]
	if !ok {
		return 0
	}
	switch typed := v.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0
		}
		return int(n)
	default:
		return 0
	}
}
