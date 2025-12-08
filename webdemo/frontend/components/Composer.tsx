/* eslint-disable @next/next/no-img-element */
"use client"

import { useState } from "react"
import clsx from "clsx"

interface ComposerProps {
  disabled: boolean
  allowImage: boolean
  onSubmit: (input: { text: string; image?: { dataUrl: string; mime: string } }) => void
}

const Composer: React.FC<ComposerProps> = ({
  disabled,
  allowImage,
  onSubmit,
}) => {
  const [text, setText] = useState("")
  const [image, setImage] = useState<{ dataUrl: string; mime: string } | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!text.trim() && !image) {
      return
    }
    onSubmit({ text: text.trim(), image: image ?? undefined })
    setText("")
    setImage(null)
    setError(null)
  }

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) {
      setImage(null)
      return
    }
    if (!allowImage) {
      setError("This provider does not support image input.")
      event.target.value = ""
      return
    }
    try {
      const dataUrl = await fileToDataUrl(file)
      setImage({ dataUrl, mime: file.type || "image/png" })
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to read image")
    }
  }

  return (
    <form className="composer" onSubmit={handleSubmit}>
      <textarea
        className="composer-input"
        placeholder="Ask anything, attach an image, or request structured JSON output."
        value={text}
        onChange={(event) => setText(event.target.value)}
        disabled={disabled}
        onKeyDown={(event) => {
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            event.currentTarget.form?.requestSubmit()
          }
        }}
      />

      <div className="composer-footer">
        <div className="composer-actions">
          <label className={clsx("ghost", "icon-button", { disabled: disabled || !allowImage })}>
            <input type="file" accept="image/*" disabled={disabled || !allowImage} onChange={handleFileChange} hidden />
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <circle cx="8.5" cy="8.5" r="1.5"/>
              <polyline points="21 15 16 10 5 21"/>
            </svg>
          </label>
          {image && (
            <div className="composer-attachment">
              <img src={image.dataUrl} alt="Attachment preview" />
              <button type="button" onClick={() => setImage(null)} className="ghost">
                Remove
              </button>
            </div>
          )}
        </div>
        <div className="composer-submit">
          {error && <span className="composer-error">{error}</span>}
          <button type="submit" className="primary" disabled={disabled}>
            Send
          </button>
        </div>
      </div>
    </form>
  )
}

async function fileToDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === "string") {
        resolve(reader.result)
      } else {
        reject(new Error("Failed to read file"))
      }
    }
    reader.onerror = () => reject(reader.error || new Error("Failed to read file"))
    reader.readAsDataURL(file)
  })
}

export default Composer
