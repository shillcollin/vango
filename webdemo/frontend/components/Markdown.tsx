"use client"

import React from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import clsx from "clsx"

interface MarkdownProps {
  content: string
  className?: string
}

const Markdown: React.FC<MarkdownProps> = ({ content, className }) => (
  <ReactMarkdown
    className={clsx("markdown-body", className)}
    remarkPlugins={[remarkGfm]}
    components={{
      code({ inline, className, children, ...props }: any) {
        const text = String(children).replace(/\n$/, "")
        if (inline) {
          return (
            <code className={clsx("md-inline-code", className)} {...props}>
              {text}
            </code>
          )
        }
        return (
          <pre className={clsx("md-code-block", className)}>
            <code {...props}>{text}</code>
          </pre>
        )
      },
      table({ className, children }) {
        return (
          <div className="md-table-wrapper">
            <table className={clsx("md-table", className)}>{children}</table>
          </div>
        )
      },
      a({ className, children, ...props }) {
        return (
          <a className={clsx("md-link", className)} target="_blank" rel="noreferrer" {...props}>
            {children}
          </a>
        )
      },
      ul({ className, children, ...props }) {
        return (
          <ul className={clsx("md-list", className)} {...props}>
            {children}
          </ul>
        )
      },
      ol({ className, children, ...props }) {
        return (
          <ol className={clsx("md-list", className)} {...props}>
            {children}
          </ol>
        )
      }
    }}
  >
    {content}
  </ReactMarkdown>
)

export default Markdown
