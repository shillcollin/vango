"use client"

import clsx from "clsx"
import { ProviderInfo } from "../lib/types"
import ProviderDropdown from "./ProviderDropdown"

interface SidebarProps {
  providers: ProviderInfo[]
  providerId: string
  onProviderChange: (id: string) => void
  model: string
  onModelChange: (model: string) => void
  selectedTools: Record<string, boolean>
  onToggleTool: (tool: string, enabled: boolean) => void
  temperature: number
  onTemperatureChange: (value: number) => void
  temperatureDisabled?: boolean
  mode: "text" | "json"
  onModeChange: (mode: "text" | "json") => void
  theme: "light" | "dark"
  onThemeToggle: () => void
  onClearChat?: () => void
}

const Sidebar: React.FC<SidebarProps> = ({
  providers,
  providerId,
  onProviderChange,
  model,
  onModelChange,
  selectedTools,
  onToggleTool,
  temperature,
  onTemperatureChange,
  temperatureDisabled = false,
  mode,
  onModeChange,
  theme,
  onThemeToggle,
  onClearChat,
}) => {
  const activeProvider = providers.find((p) => p.id === providerId)
  const models = activeProvider?.models ?? []
  const tools = activeProvider?.tools ?? []

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="sidebar-brand">
          <span className="sidebar-title">GAI Demo</span>
        </div>
        <p className="sidebar-subtitle">Orchestrate OpenAI, Anthropic, and Gemini from one polished UI.</p>
      </div>

      <section className="sidebar-section">
        <h3>Provider</h3>
        <ProviderDropdown
          providers={providers}
          selectedProviderId={providerId}
          onProviderChange={onProviderChange}
        />
      </section>

      <section className="sidebar-section">
        <h3>Model</h3>
        <div className="sidebar-pill-group" role="radiogroup">
          {models.map((candidate) => (
            <button
              key={candidate}
              type="button"
              role="radio"
              aria-checked={candidate === model}
              className={clsx("pill", { active: candidate === model })}
              onClick={() => onModelChange(candidate)}
            >
              {candidate}
            </button>
          ))}
        </div>
      </section>

      {tools.length > 0 && (
        <section className="sidebar-section">
          <h3>Tools</h3>
          <div className="sidebar-pill-group">
            {/* Consolidate web_search and url_extract into one Search button */}
            {tools.includes("web_search") && tools.includes("url_extract") ? (
              <button
                type="button"
                className={clsx("pill", {
                  active: selectedTools["web_search"] && selectedTools["url_extract"]
                })}
                onClick={() => {
                  const newState = !(selectedTools["web_search"] && selectedTools["url_extract"])
                  onToggleTool("web_search", newState)
                  onToggleTool("url_extract", newState)
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  style={{ marginRight: "0.4rem", verticalAlign: "middle" }}
                >
                  <circle cx="11" cy="11" r="8"/>
                  <path d="m21 21-4.35-4.35"/>
                </svg>
                Search
              </button>
            ) : (
              <>
                {/* Show individual tool buttons for non-web search tools */}
                {tools.map((tool) => {
                  if (tool === "web_search" || tool === "url_extract") return null
                  return (
                    <button
                      key={tool}
                      type="button"
                      className={clsx("pill", { active: selectedTools[tool] })}
                      onClick={() => onToggleTool(tool, !selectedTools[tool])}
                    >
                      {tool}
                    </button>
                  )
                })}
                {/* If only one of the web tools is present, show it individually */}
                {tools.includes("web_search") && !tools.includes("url_extract") && (
                  <button
                    type="button"
                    className={clsx("pill", { active: selectedTools["web_search"] })}
                    onClick={() => onToggleTool("web_search", !selectedTools["web_search"])}
                  >
                    web_search
                  </button>
                )}
                {!tools.includes("web_search") && tools.includes("url_extract") && (
                  <button
                    type="button"
                    className={clsx("pill", { active: selectedTools["url_extract"] })}
                    onClick={() => onToggleTool("url_extract", !selectedTools["url_extract"])}
                  >
                    url_extract
                  </button>
                )}
              </>
            )}
          </div>
        </section>
      )}

      <section className="sidebar-section">
        <h3>Mode</h3>
        <div className="sidebar-pill-group" role="radiogroup">
          <button
            type="button"
            role="radio"
            aria-checked={mode === "text"}
            className={clsx("pill", { active: mode === "text" })}
            onClick={() => onModeChange("text")}
          >
            Conversational
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={mode === "json"}
            className={clsx("pill", { active: mode === "json" })}
            onClick={() => onModeChange("json")}
          >
            Structured JSON
          </button>
        </div>
      </section>

      <section className="sidebar-section">
        <h3>Parameters</h3>
        <div className="sidebar-slider">
          <label htmlFor="sidebar-temperature" className="sidebar-slider-label">
            <span>Temperature</span>
            <span>{temperature.toFixed(1)}</span>
          </label>
          <input
            id="sidebar-temperature"
            type="range"
            min={0}
            max={2}
            step={0.1}
            value={temperature}
            onChange={(event) => onTemperatureChange(parseFloat(event.target.value))}
            disabled={temperatureDisabled}
          />
        </div>
      </section>

      <section className="sidebar-spacer" aria-hidden="true" />

      <section className="sidebar-section sidebar-footer">
        <div className="sidebar-footer-controls">
          <button
            type="button"
            className="ghost icon-button"
            onClick={onThemeToggle}
            aria-label={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
          >
            {theme === "dark" ? (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="5"/>
                <line x1="12" y1="1" x2="12" y2="3"/>
                <line x1="12" y1="21" x2="12" y2="23"/>
                <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
                <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
                <line x1="1" y1="12" x2="3" y2="12"/>
                <line x1="21" y1="12" x2="23" y2="12"/>
                <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
                <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
              </svg>
            ) : (
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
              </svg>
            )}
          </button>
          {onClearChat && (
            <button type="button" className="ghost" onClick={onClearChat}>
              Clear chat
            </button>
          )}
        </div>
        <p className="sidebar-footnote">Made with the GAI Go SDK — unified tools, structured output, and observability out of the box.</p>
      </section>
    </aside>
  )
}

export default Sidebar
