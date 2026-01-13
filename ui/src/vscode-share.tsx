import React from 'react'
import ReactDOM from 'react-dom/client'
import { ShareMenu, type ShareMenuPort, type ShareMenuTheme } from '@/components/ShareMenu'
import './index.css'

// Global function to render the ShareMenu
declare global {
  interface Window {
    renderShareMenu: (
      containerId: string,
      port: ShareMenuPort,
      theme: ShareMenuTheme,
      onShare: (mode: string, password?: string) => void,
      onClose: () => void,
      onCopyUrl: () => void
    ) => void
    unmountShareMenu: (containerId: string) => void
  }
}

const roots = new Map<string, ReactDOM.Root>()

window.renderShareMenu = (
  containerId: string,
  port: ShareMenuPort,
  theme: ShareMenuTheme,
  onShare: (mode: string, password?: string) => void,
  onClose: () => void,
  onCopyUrl: () => void
) => {
  const container = document.getElementById(containerId)
  if (!container) {
    console.error(`Container ${containerId} not found`)
    return
  }

  // Unmount existing if any
  if (roots.has(containerId)) {
    roots.get(containerId)!.unmount()
  }

  const root = ReactDOM.createRoot(container)
  roots.set(containerId, root)

  root.render(
    <React.StrictMode>
      <ShareMenu
        port={port}
        theme={theme}
        onShare={onShare}
        onClose={onClose}
        onCopyUrl={onCopyUrl}
      />
    </React.StrictMode>
  )
}

window.unmountShareMenu = (containerId: string) => {
  if (roots.has(containerId)) {
    roots.get(containerId)!.unmount()
    roots.delete(containerId)
  }
}
