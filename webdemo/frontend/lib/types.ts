export interface ProviderCapabilities {
  Streaming?: boolean
  ParallelToolCalls?: boolean
  StrictJSON?: boolean
  Images?: boolean
  Audio?: boolean
  Video?: boolean
  Files?: boolean
  Reasoning?: boolean
  Citations?: boolean
  Safety?: boolean
  Sessions?: boolean
  MaxInputTokens?: number
  MaxOutputTokens?: number
  MaxFileSize?: number
  MaxToolCalls?: number
  Provider?: string
  Models?: string[]
  [key: string]: unknown
}

export interface ProviderInfo {
  id: string
  label: string
  default_model: string
  models: string[]
  capabilities: ProviderCapabilities
  tools: string[]
  system_prompt?: string
  prompt_metadata?: Record<string, string>
}

export type Role = "system" | "user" | "assistant"

export interface Usage {
  input_tokens: number
  output_tokens: number
  total_tokens: number
  reasoning_tokens?: number
}

export type MessagePart =
  | { type: "text"; text: string }
  | { type: "image"; dataUrl: string; mime: string }

export interface ReasoningEvent {
  id: string
  text: string
  kind: "thinking" | "summary"
  step?: number
  timestamp: number
}

export interface ToolCallEvent {
  id: string
  name: string
  status: "awaiting" | "running" | "completed"
  input?: Record<string, unknown>
  result?: unknown
  error?: string
  duration_ms?: number
  retries?: number
  metadata?: Record<string, unknown>
  step?: number
  timestamp: number
}

export interface ChatMessage {
  id: string
  role: Role
  createdAt: number
  parts: MessagePart[]
  content: string
  reasoning: ReasoningEvent[]
  toolCalls: ToolCallEvent[]
  usage?: Usage
  finishReason?: string
  model?: string
  provider?: string
  warnings?: string[]
  status: "streaming" | "complete"
}

export interface StreamEventPayload {
  type: string
  step?: number
  seq: number
  ts: number
  provider?: string
  model?: string
  text_delta?: string
  reasoning_delta?: string
  reasoning_summary?: string
  tool_call?: ToolEventPayload
  tool_result?: ToolEventPayload
  usage?: Usage
  finish_reason?: {
    type: string
    description?: string
  }
  ext?: Record<string, unknown>
}

export interface WarningDTO {
  code: string
  field?: string
  message: string
}

export interface ToolEventPayload {
  id: string
  name: string
  input?: Record<string, unknown>
  result?: unknown
  error?: string
  duration_ms?: number
  retries?: number
  metadata?: Record<string, unknown>
}

export interface ChatRequestPayload {
  provider: string
  model?: string
  mode?: string
  messages: ApiMessage[]
  temperature?: number
  max_output_tokens?: number
  tool_choice?: string
  tools?: string[]
  provider_options?: Record<string, unknown>
}

export interface ApiMessage {
  role: Role
  parts: {
    type: string
    text?: string
    data?: string
    mime?: string
  }[]
}
