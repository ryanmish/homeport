import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { DesignPreview } from './DesignPreview.tsx'

// Simple URL-based routing
function Router() {
  const path = window.location.pathname

  if (path === '/design') {
    return <DesignPreview />
  }

  return <App />
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <Router />
  </StrictMode>,
)
