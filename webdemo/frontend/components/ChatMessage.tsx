/* eslint-disable @next/next/no-img-element */
"use client"

import clsx from "clsx"
import Markdown from "./Markdown"
import { ChatMessage as ChatMessageType, MessagePart, ReasoningEvent, ToolCallEvent } from "../lib/types"

interface ChatMessageProps {
  message: ChatMessageType
}

const ChatMessage: React.FC<ChatMessageProps> = ({ message }) => {
  const isAssistant = message.role === "assistant"

  return (
    <article className={clsx("chat-message", message.role)}>
      {message.role === "assistant" && (
        <header className="chat-message-header">
          <div className="chat-message-meta">
            {message.provider && message.model && (
              <span className="chat-model">{message.provider} • {message.model}</span>
            )}
          </div>
          {message.status === "streaming" && <span className="chat-status">Streaming…</span>}
        </header>
      )}

      {renderParts(message.parts)}

      {isAssistant && message.content && message.content.trim().length > 0 && (
        <Markdown content={message.content} className="chat-markdown" />
      )}

      {isAssistant && message.reasoning.length > 0 && renderReasoning(message.reasoning)}

      {isAssistant && message.toolCalls.length > 0 && renderTools(message.toolCalls)}

      {message.usage && (
        <footer className="chat-usage">
          tokens in {message.usage.input_tokens} • out {message.usage.output_tokens}
          {typeof message.usage.reasoning_tokens === "number" && message.usage.reasoning_tokens > 0
            ? ` • reasoning ${message.usage.reasoning_tokens}`
            : null}
        </footer>
      )}

      {message.warnings && message.warnings.length > 0 && (
        <ul className="chat-warnings">
          {message.warnings.map((warning, idx) => (
            <li key={idx}>{warning}</li>
          ))}
        </ul>
      )}
    </article>
  )
}

function renderParts(parts: MessagePart[]) {
  if (!parts || parts.length === 0) {
    return null
  }
  return (
    <div className="chat-message-parts">
      {parts.map((part, idx) => {
        if (part.type === "text") {
          return (
            <p key={idx} className="chat-text">
              {part.text}
            </p>
          )
        }
        return <img key={idx} src={part.dataUrl} alt="User upload" className="chat-image" />
      })}
    </div>
  )
}

function renderReasoning(reasoning: ReasoningEvent[]) {
  return (
    <details className="chat-reasoning">
      <summary>Reasoning trace</summary>
      <div className="chat-reasoning-body">
        {reasoning.map((chunk) => (
          <div key={chunk.id} className={clsx("chat-reasoning-item", chunk.kind)}>
            {chunk.kind === "summary" && <span className="badge">Summary</span>}
            <p>{chunk.text}</p>
          </div>
        ))}
      </div>
    </details>
  )
}

function renderTools(tools: ToolCallEvent[]) {
  return (
    <div className="chat-tools">
      {tools.map((tool) => (
        <details key={tool.id} className={clsx("tool-card", tool.status)}>
          <summary>
            <strong>{tool.name}</strong>
            <span className="tool-status">{labelForToolStatus(tool.status)}</span>
          </summary>
          <div className="tool-card-content">
            {tool.input && (
              <div className="tool-section">
                <span className="tool-section-label">Input</span>
                <pre>{JSON.stringify(tool.input, null, 2)}</pre>
              </div>
            )}
            {tool.result !== undefined && tool.result !== null && (
              <div className="tool-section">
                <span className="tool-section-label">Result</span>
                <pre>{JSON.stringify(tool.result, null, 2)}</pre>
              </div>
            )}
            {tool.error && <div className="tool-error">{tool.error}</div>}
            {(tool.duration_ms || tool.retries) && (
              <footer className="tool-footer">
                {tool.duration_ms ? `${tool.duration_ms} ms` : null}
                {tool.retries ? ` • ${tool.retries} retries` : null}
              </footer>
            )}
          </div>
        </details>
      ))}
    </div>
  )
}

function labelForToolStatus(status: ToolCallEvent["status"]) {
  switch (status) {
    case "awaiting":
      return "Queued"
    case "running":
      return "Running"
    case "completed":
      return "Completed"
    default:
      return ""
  }
}

export default ChatMessage
