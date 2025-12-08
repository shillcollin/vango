"use client"

import React, { useState, useRef, useEffect } from "react"
import clsx from "clsx"

interface Provider {
  id: string
  label: string
  capabilities: {
    Streaming?: boolean
    Reasoning?: boolean
  }
}

interface ProviderDropdownProps {
  providers: Provider[]
  selectedProviderId: string
  onProviderChange: (id: string) => void
}

const providerLogos: Record<string, JSX.Element> = {
  openai: (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.8956zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"/>
    </svg>
  ),
  "openai-chat": (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.8956zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"/>
    </svg>
  ),
  "openai-responses": (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729zm-9.022 12.6081a4.4755 4.4755 0 0 1-2.8764-1.0408l.1419-.0804 4.7783-2.7582a.7948.7948 0 0 0 .3927-.6813v-6.7369l2.02 1.1686a.071.071 0 0 1 .038.052v5.5826a4.504 4.504 0 0 1-4.4945 4.4944zm-9.6607-4.1254a4.4708 4.4708 0 0 1-.5346-3.0137l.142.0852 4.783 2.7582a.7712.7712 0 0 0 .7806 0l5.8428-3.3685v2.3324a.0804.0804 0 0 1-.0332.0615L9.74 19.9502a4.4992 4.4992 0 0 1-6.1408-1.6464zM2.3408 7.8956a4.485 4.485 0 0 1 2.3655-1.9728V11.6a.7664.7664 0 0 0 .3879.6765l5.8144 3.3543-2.0201 1.1685a.0757.0757 0 0 1-.071 0l-4.8303-2.7865A4.504 4.504 0 0 1 2.3408 7.8956zm16.5963 3.8558L13.1038 8.364 15.1192 7.2a.0757.0757 0 0 1 .071 0l4.8303 2.7913a4.4944 4.4944 0 0 1-.6765 8.1042v-5.6772a.79.79 0 0 0-.407-.667zm2.0107-3.0231l-.142-.0852-4.7735-2.7818a.7759.7759 0 0 0-.7854 0L9.409 9.2297V6.8974a.0662.0662 0 0 1 .0284-.0615l4.8303-2.7866a4.4992 4.4992 0 0 1 6.6802 4.66zM8.3065 12.863l-2.02-1.1638a.0804.0804 0 0 1-.038-.0567V6.0742a4.4992 4.4992 0 0 1 7.3757-3.4537l-.142.0805L8.704 5.459a.7948.7948 0 0 0-.3927.6813zm1.0976-2.3654l2.602-1.4998 2.6069 1.4998v2.9994l-2.5974 1.4997-2.6067-1.4997Z"/>
    </svg>
  ),
  anthropic: (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M17.895 5.4h-3.79l-6 13.2h3.79l1.2-2.755h5.4l1.2 2.755h3.79l-5.59-13.2zm-3.395 7.845l1.5-3.445 1.5 3.445h-3zM6 5.4L0 18.6h3.79l6-13.2H6z"/>
    </svg>
  ),
  gemini: (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2C12 7.52285 16.4772 12 22 12C16.4772 12 12 16.4772 12 22C12 16.4772 7.52285 12 2 12C7.52285 12 12 7.52285 12 2Z"/>
    </svg>
  ),
  groq: (
    <svg width="20" height="20" viewBox="0 0 11 15" fill="currentColor">
      <path d="M10.2739 5.00225C10.2406 3.65803 9.69276 2.40012 8.73149 1.46036C7.7711 0.521495 6.50331 0.00266977 5.16188 0H5.1184C2.31666 0 0.0245998 2.27242 0.000202081 5.08635C-0.011775 6.45683 0.509006 7.75034 1.46673 8.72881C2.42489 9.70772 3.70467 10.2532 5.07493 10.2653H6.63639V7.54654H5.15389C4.51112 7.55544 3.90384 7.30982 3.44428 6.85997C2.98427 6.40967 2.72698 5.8063 2.71988 5.16066C2.70525 3.82978 3.77076 2.73473 5.09622 2.71871H5.1601C6.48335 2.71871 7.56572 3.80175 7.57326 5.1304V9.87617C7.57326 11.1897 6.5042 12.2709 5.18982 12.2879C4.56036 12.283 3.9686 12.0329 3.52368 11.5826L3.17856 11.2337L3.17723 11.2351L1.79011 13.685C2.71589 14.53 3.90162 14.9972 5.1601 15.0066H5.2293C6.57828 14.9874 7.84385 14.4477 8.79315 13.4861C9.74155 12.525 10.2681 11.2597 10.2862 9.90903V5.00225H10.2739Z"/>
    </svg>
  ),
  xai: (
    <svg width="20" height="20" viewBox="0 0 841.89 595.28" fill="currentColor">
      <polygon points="557.09,211.99 565.4,538.36 631.96,538.36 640.28,93.18"/>
      <polygon points="640.28,56.91 538.72,56.91 379.35,284.53 430.13,357.05"/>
      <polygon points="201.61,538.36 303.17,538.36 353.96,465.84 303.17,393.31"/>
      <polygon points="201.61,211.99 430.13,538.36 531.69,538.36 303.17,211.99"/>
    </svg>
  )
}

const ProviderDropdown: React.FC<ProviderDropdownProps> = ({
  providers,
  selectedProviderId,
  onProviderChange,
}) => {
  const [isOpen, setIsOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)

  const selectedProvider = providers.find(p => p.id === selectedProviderId)

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false)
      }
    }

    document.addEventListener("mousedown", handleClickOutside)
    return () => {
      document.removeEventListener("mousedown", handleClickOutside)
    }
  }, [])

  const handleSelect = (providerId: string) => {
    onProviderChange(providerId)
    setIsOpen(false)
  }

  const getProviderLogo = (providerId: string) => {
    const normalizedId = providerId.toLowerCase()
    return providerLogos[normalizedId] || null
  }

  return (
    <div className="provider-dropdown" ref={dropdownRef}>
      <button
        type="button"
        className={clsx("provider-dropdown-trigger", { open: isOpen })}
        onClick={() => setIsOpen(!isOpen)}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
      >
        <div className="provider-dropdown-selected">
          <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
            <span className="provider-logo">
              {selectedProvider && getProviderLogo(selectedProvider.id)}
            </span>
            <span className="provider-label">
              {selectedProvider?.label || "Select Provider"}
            </span>
          </div>
        </div>
        <svg
          className="dropdown-chevron"
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {isOpen && (
        <div className="provider-dropdown-menu" role="listbox">
          {providers.map((provider) => (
            <button
              key={provider.id}
              type="button"
              className={clsx("provider-dropdown-item", {
                selected: provider.id === selectedProviderId
              })}
              onClick={() => handleSelect(provider.id)}
              role="option"
              aria-selected={provider.id === selectedProviderId}
            >
              <span className="provider-logo">
                {getProviderLogo(provider.id)}
              </span>
              <span className="provider-label">{provider.label}</span>
              {provider.id === selectedProviderId && (
                <svg
                  className="checkmark"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

export default ProviderDropdown