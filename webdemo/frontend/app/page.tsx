"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import clsx from "clsx"
import Sidebar from "../components/Sidebar"
import ChatMessage from "../components/ChatMessage"
import Composer from "../components/Composer"
import {
  ApiMessage,
  ChatMessage as ChatMessageType,
  ChatRequestPayload,
  MessagePart,
  ProviderInfo,
  ReasoningEvent,
  StreamEventPayload,
  ToolCallEvent,
  Usage,
  WarningDTO,
} from "../lib/types"

const API_BASE = process.env.NEXT_PUBLIC_GAI_API_BASE ?? "http://localhost:8080"
type ThemeMode = "light" | "dark"

type ChatMode = "text" | "json"

type StreamResult = {
  text: string
  usage?: Usage
  finishReason?: string
  provider?: string
  model?: string
  warnings: string[]
}

export default function Page() {
  const [providers, setProviders] = useState<ProviderInfo[]>([])
  const [providerId, setProviderId] = useState<string>("")
  const [model, setModel] = useState<string>("")
  const [selectedTools, setSelectedTools] = useState<Record<string, boolean>>({})
  const [systemPrompt, setSystemPrompt] = useState<string>("")
  const [conversation, setConversation] = useState<ApiMessage[]>([])
  const [messages, setMessages] = useState<ChatMessageType[]>([])
  const [temperature, setTemperature] = useState<number>(0.7)
  const [mode, setMode] = useState<ChatMode>("text")
  const [loading, setLoading] = useState<boolean>(false)
  const [error, setError] = useState<string | null>(null)
  const [theme, setTheme] = useState<ThemeMode>("dark")
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false)
  const threadRef = useRef<HTMLDivElement | null>(null)

  const makeSystemMessage = useCallback(
    (prompt: string): ApiMessage => ({
      role: "system",
      parts: [{ type: "text", text: prompt }],
    }),
    [],
  )

  const activeProvider = useMemo(
    () => providers.find((provider) => provider.id === providerId) ?? null,
    [providers, providerId],
  )

  const providerLabel = activeProvider?.label ?? providerId ?? ""

  const enabledTools = useMemo(
    () => Object.entries(selectedTools).filter(([, enabled]) => enabled).map(([name]) => name),
    [selectedTools],
  )

  const allowImages = useMemo(() => {
    if (!activeProvider) {
      return false
    }
    const caps = activeProvider.capabilities
    return Boolean((caps as any).Images ?? (caps as any).images)
  }, [activeProvider])

  useEffect(() => {
    async function loadProviders() {
      try {
        const response = await fetch(`${API_BASE}/api/providers`)
        if (!response.ok) {
          throw new Error(`Request failed with status ${response.status}`)
        }
        const payload: ProviderInfo[] = await response.json()
        setProviders(payload)
        if (payload.length > 0) {
          const first = payload[0]
          const promptText = first.system_prompt ?? ""
          setSystemPrompt(promptText)
          setProviderId(first.id)
          setModel(first.default_model)
          resetSession(first, promptText)
        }
      } catch (err) {
        console.error("Failed to load providers", err)
        setError("Failed to load providers. Is the API server running?")
      }
    }
    loadProviders()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (typeof window === "undefined") {
      return
    }
    const saved = window.localStorage.getItem("gai-demo-theme") as ThemeMode | null
    if (saved) {
      setTheme(saved)
    } else {
      const prefersLight = window.matchMedia("(prefers-color-scheme: light)").matches
      setTheme(prefersLight ? "light" : "dark")
    }
  }, [])

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.body.dataset.theme = theme
      window.localStorage.setItem("gai-demo-theme", theme)
    }
  }, [theme])

  useEffect(() => {
    if (!threadRef.current) {
      return
    }
    threadRef.current.scrollTo({ top: threadRef.current.scrollHeight, behavior: "smooth" })
  }, [messages])

  const handleProviderChange = (id: string) => {
    const next = providers.find((provider) => provider.id === id)
    if (!next) {
      return
    }
    setProviderId(id)
    setError(null)
    const nextPrompt = next.system_prompt ?? ""
    setSystemPrompt(nextPrompt)
    setConversation((prev) => {
      if (prev.length === 0) {
        return nextPrompt ? [makeSystemMessage(nextPrompt)] : prev
      }
      const [first, ...rest] = prev
      if (first.role !== "system") {
        return nextPrompt ? [makeSystemMessage(nextPrompt), ...prev] : prev
      }
      const firstPart = first.parts[0]
      const firstText = typeof firstPart?.text === "string" ? firstPart.text : ""
      if (firstText.trim() === nextPrompt.trim()) {
        return prev
      }
      return nextPrompt ? [makeSystemMessage(nextPrompt), ...rest] : rest
    })
    setModel((current) => (next.models.includes(current) ? current : next.default_model))
    setSelectedTools((prev) => {
      if (next.tools.length === 0) {
        return {}
      }
      const defaults = defaultToolSelection(next.tools)
      const merged: Record<string, boolean> = { ...defaults }
      for (const [name, enabled] of Object.entries(prev)) {
        if (name in defaults) {
          merged[name] = enabled
        }
      }
      return merged
    })
  }

  const handleToolToggle = (tool: string, enabled: boolean) => {
    setSelectedTools((prev) => ({ ...prev, [tool]: enabled }))
  }

  const handleThemeToggle = () => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"))
  }

  const resetSession = useCallback(
    (provider?: ProviderInfo, promptOverride?: string) => {
      const promptText = (promptOverride ?? systemPrompt).trim()
      setConversation(promptText ? [makeSystemMessage(promptText)] : [])
      setMessages([])
      setError(null)
      if (provider) {
        setSelectedTools(defaultToolSelection(provider.tools))
      }
    },
    [makeSystemMessage, systemPrompt],
  )

  const handleClearChat = () => {
    resetSession(activeProvider ?? undefined)
  }

  const runBatchRequest = useCallback(
    async (request: ChatRequestPayload, assistantId: string): Promise<StreamResult> => {
      const response = await fetch(`${API_BASE}/api/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      })
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}))
        throw new Error((payload as any).error || `Request failed with status ${response.status}`)
      }
      const payload = await response.json()
      const warnings = Array.isArray(payload.warnings)
        ? (payload.warnings as WarningDTO[]).map((warning) => warning.message)
        : []
      const jsonText = payload.json ? JSON.stringify(payload.json, null, 2) : undefined
      const messageText = payload.text ?? (jsonText ? `\`\`\`json\n${jsonText}\n\`\`\`` : "")

      setMessages((prev) =>
        prev.map((msg) =>
          msg.id === assistantId
            ? {
                ...msg,
                content: messageText,
                toolCalls: [],
                reasoning: [],
                warnings,
                status: "complete",
                provider: payload.provider ?? msg.provider,
                model: payload.model ?? msg.model,
                usage: payload.usage ?? msg.usage,
              }
            : msg,
        ),
      )

      return {
        text: messageText,
        usage: payload.usage,
        finishReason: payload.finish_reason?.type,
        provider: payload.provider ?? providerLabel,
        model: payload.model ?? model,
        warnings,
      }
    },
    [model, providerLabel],
  )

  const runStreamingRequest = useCallback(
    async (request: ChatRequestPayload, assistantId: string): Promise<StreamResult> => {
      const response = await fetch(`${API_BASE}/api/chat/stream`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      })
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}))
        throw new Error((payload as any).error || `Request failed with status ${response.status}`)
      }
      if (!response.body) {
        throw new Error("Streaming is not supported by this server")
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ""
      let assistantText = ""
      let providerName: string | undefined = providerLabel
      let modelName: string | undefined = model
      let usage: Usage | undefined
      let finishReason: string | undefined

      const applyUpdate = (updater: (message: ChatMessageType) => ChatMessageType) => {
        setMessages((prev) =>
          prev.map((msg) => {
            if (msg.id !== assistantId) {
              return msg
            }
            return updater({ ...msg })
          }),
        )
      }

      while (true) {
        const { value, done } = await reader.read()
        buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done })

        let newlineIndex = buffer.indexOf("\n")
        while (newlineIndex !== -1) {
          const line = buffer.slice(0, newlineIndex).trim()
          buffer = buffer.slice(newlineIndex + 1)
          if (line) {
            const event = JSON.parse(line) as StreamEventPayload
            ;({
              assistantText,
              providerName,
              modelName,
              usage,
              finishReason,
            } = handleStreamEvent({
              event,
              assistantId,
              assistantText,
              providerName,
              modelName,
              usage,
              finishReason,
              applyUpdate,
            }))
          }
          newlineIndex = buffer.indexOf("\n")
        }

        if (done) {
          const tail = buffer.trim()
          if (tail) {
            const event = JSON.parse(tail) as StreamEventPayload
            ;({
              assistantText,
              providerName,
              modelName,
              usage,
              finishReason,
            } = handleStreamEvent({
              event,
              assistantId,
              assistantText,
              providerName,
              modelName,
              usage,
              finishReason,
              applyUpdate,
            }))
          }
          break
        }
      }

      applyUpdate((msg) => ({
        ...msg,
        content: assistantText,
        status: "complete",
        provider: providerName ?? msg.provider,
        model: modelName ?? msg.model,
        usage: usage ?? msg.usage,
        finishReason: finishReason ?? msg.finishReason,
      }))

      return {
        text: assistantText,
        usage,
        finishReason,
        provider: providerName,
        model: modelName,
        warnings: [],
      }
    },
    [model, providerLabel],
  )

  const handleSend = useCallback(
    async ({ text, image }: { text: string; image?: { dataUrl: string; mime: string } }) => {
      if (!providerId) {
        setError("Select a provider to get started.")
        return
      }
      if (loading) {
        return
      }
      setLoading(true)
      setError(null)

      const timestamp = Date.now()
      const assistantId = crypto.randomUUID()
      const userParts = buildMessageParts(text, image)

      const userMessage: ChatMessageType = {
        id: crypto.randomUUID(),
        role: "user",
        createdAt: timestamp,
        parts: userParts,
        content: text,
        reasoning: [],
        toolCalls: [],
        status: "complete",
      }

      const assistantMessage: ChatMessageType = {
        id: assistantId,
        role: "assistant",
        createdAt: timestamp,
        parts: [],
        content: "",
        reasoning: [],
        toolCalls: [],
        provider: providerLabel,
        model,
        status: "streaming",
      }

      setMessages((prev) => [...prev, userMessage, assistantMessage])

      const apiUserMessage: ApiMessage = {
        role: "user",
        parts: buildApiParts(text, image),
      }

      const request: ChatRequestPayload = {
        provider: providerId,
        model,
        mode,
        messages: [...conversation, apiUserMessage],
        temperature,
        tool_choice: enabledTools.length > 0 ? "auto" : "none",
        tools: enabledTools,
      }

      try {
        let result: StreamResult
        if (mode === "json") {
          result = await runBatchRequest(request, assistantId)
        } else {
          result = await runStreamingRequest(request, assistantId)
        }

        const assistantApiMessage: ApiMessage = {
          role: "assistant",
          parts: result.text
            ? [
                {
                  type: "text",
                  text: result.text,
                },
              ]
            : [],
        }
        setConversation((prev) => [...prev, apiUserMessage, assistantApiMessage])

        setMessages((prev) =>
          prev.map((msg) => {
            if (msg.id !== assistantId) {
              return msg
            }
            return {
              ...msg,
              status: "complete",
              content: result.text,
              usage: result.usage ?? msg.usage,
              finishReason: result.finishReason ?? msg.finishReason,
              provider: result.provider ?? msg.provider,
              model: result.model ?? msg.model,
              warnings: result.warnings.length > 0 ? result.warnings : msg.warnings,
            }
          }),
        )
      } catch (err) {
        const message = err instanceof Error ? err.message : "Unexpected error"
        setError(message)
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === assistantId
              ? {
                  ...msg,
                  status: "complete",
                  content: msg.content || `⚠️ ${message}`,
                }
              : msg,
          ),
        )
      } finally {
        setLoading(false)
      }
    },
    [
      providerId,
      conversation,
      enabledTools,
      loading,
      model,
      mode,
      providerLabel,
      temperature,
      runBatchRequest,
      runStreamingRequest,
    ],
  )

  return (
    <div className={clsx("app-shell", { "sidebar-hidden": sidebarCollapsed })}>
      {!sidebarCollapsed && (
        <Sidebar
          providers={providers}
          providerId={providerId}
          onProviderChange={handleProviderChange}
          model={model}
          onModelChange={setModel}
          selectedTools={selectedTools}
          onToggleTool={handleToolToggle}
          temperature={temperature}
          onTemperatureChange={setTemperature}
          temperatureDisabled={loading || !providerId}
          mode={mode}
          onModeChange={setMode}
          theme={theme}
          onThemeToggle={handleThemeToggle}
          onClearChat={messages.length > 0 ? handleClearChat : undefined}
        />
      )}

      <main className="main">
        <div className="main-toolbar">
          <button
            type="button"
            className="sidebar-toggle"
            onClick={() => setSidebarCollapsed((prev) => !prev)}
            aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {sidebarCollapsed ? (
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3.5" y="4" width="16.5" height="16" rx="2.2" />
                <path d="M15.5 4v16" />
                <polyline points="11 8 15.5 12 11 16" />
              </svg>
            ) : (
              <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <rect x="3.5" y="4" width="16.5" height="16" rx="2.2" />
                <path d="M8.5 4v16" />
                <polyline points="13 8 8.5 12 13 16" />
              </svg>
            )}
          </button>
        </div>

        {error && (
          <div className="alert" role="status">
            <span>{error}</span>
            <button type="button" onClick={() => setError(null)}>
              Dismiss
            </button>
          </div>
        )}

        <div className="chat-thread" ref={threadRef}>
          {messages.length === 0 ? (
            <div className="empty-state">
              <h2>Start a conversation</h2>
              <p>
                Choose a provider on the left, then ask a question, request structured JSON, or try a tool-enabled task.
                Image input is available when supported.
              </p>
            </div>
          ) : (
            messages.map((message) => <ChatMessage key={message.id} message={message} />)
          )}
        </div>

        <Composer
          disabled={loading || !providerId}
          allowImage={allowImages}
          onSubmit={handleSend}
        />
      </main>
    </div>
  )
}

function defaultToolSelection(tools: string[]) {
  return tools.reduce<Record<string, boolean>>((acc, tool) => {
    acc[tool] = true
    return acc
  }, {})
}

function buildMessageParts(text: string, image?: { dataUrl: string; mime: string }): MessagePart[] {
  const parts: MessagePart[] = []
  if (text.trim()) {
    parts.push({ type: "text", text: text.trim() })
  }
  if (image) {
    parts.push({ type: "image", dataUrl: image.dataUrl, mime: image.mime })
  }
  return parts
}

function buildApiParts(text: string, image?: { dataUrl: string; mime: string }) {
  const parts: ApiMessage["parts"] = []
  if (text.trim()) {
    parts.push({ type: "text", text: text.trim() })
  }
  if (image) {
    const [, base64] = image.dataUrl.split(",", 2)
    parts.push({ type: "image_base64", data: base64 ?? image.dataUrl, mime: image.mime })
  }
  return parts
}

function handleStreamEvent({
  event,
  assistantId,
  assistantText,
  providerName,
  modelName,
  usage,
  finishReason,
  applyUpdate,
}: {
  event: StreamEventPayload
  assistantId: string
  assistantText: string
  providerName?: string
  modelName?: string
  usage?: Usage
  finishReason?: string
  applyUpdate: (updater: (message: ChatMessageType) => ChatMessageType) => void
}) {
  let nextText = assistantText
  let nextProvider = providerName
  let nextModel = modelName
  let nextUsage = usage
  let nextFinish = finishReason

  if (event.provider) {
    nextProvider = event.provider
  }
  if (event.model) {
    nextModel = event.model
  }

  switch (event.type) {
    case "text.delta":
      if (event.text_delta) {
        nextText += event.text_delta
        applyUpdate((msg) => ({ ...msg, content: nextText }))
      }
      break
    case "reasoning.delta":
      if (event.reasoning_delta) {
        const reasoning: ReasoningEvent = {
          id: crypto.randomUUID(),
          text: event.reasoning_delta,
          kind: "thinking",
          step: event.step,
          timestamp: Date.now(),
        }
        applyUpdate((msg) => ({ ...msg, reasoning: [...msg.reasoning, reasoning] }))
      }
      break
    case "reasoning.summary":
      if (event.reasoning_summary) {
        const summary: ReasoningEvent = {
          id: crypto.randomUUID(),
          text: event.reasoning_summary,
          kind: "summary",
          step: event.step,
          timestamp: Date.now(),
        }
        applyUpdate((msg) => ({ ...msg, reasoning: [...msg.reasoning, summary] }))
      }
      break
    case "tool.call":
      if (event.tool_call) {
        const tool: ToolCallEvent = {
          id: event.tool_call.id,
          name: event.tool_call.name,
          status: "running",
          input: event.tool_call.input ?? undefined,
          metadata: event.tool_call.metadata ?? undefined,
          timestamp: Date.now(),
          step: event.step,
        }
        applyUpdate((msg) => ({ ...msg, toolCalls: mergeToolCall(msg.toolCalls, tool) }))
      }
      break
    case "tool.result":
      if (event.tool_result) {
        const tool: ToolCallEvent = {
          id: event.tool_result.id,
          name: event.tool_result.name,
          status: "completed",
          input: undefined,
          result: event.tool_result.result,
          error: event.tool_result.error ?? undefined,
          duration_ms: event.tool_result.duration_ms ?? undefined,
          retries: event.tool_result.retries ?? undefined,
          metadata: event.tool_result.metadata ?? undefined,
          timestamp: Date.now(),
          step: event.step,
        }
        applyUpdate((msg) => ({ ...msg, toolCalls: mergeToolCall(msg.toolCalls, tool) }))
      }
      break
    case "finish":
      nextUsage = event.usage ?? nextUsage
      nextFinish = event.finish_reason?.type ?? nextFinish
      break
    default:
      break
  }

  return {
    assistantText: nextText,
    providerName: nextProvider,
    modelName: nextModel,
    usage: nextUsage,
    finishReason: nextFinish,
  }
}

function mergeToolCall(existing: ToolCallEvent[], incoming: ToolCallEvent) {
  const index = existing.findIndex((tool) => tool.id === incoming.id)
  if (index === -1) {
    return [...existing, incoming]
  }
  const updated = [...existing]
  updated[index] = {
    ...updated[index],
    ...incoming,
    input: incoming.input ?? updated[index].input,
  }
  return updated
}
