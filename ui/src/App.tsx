import { useEffect, useState, useCallback, useRef, createContext } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { api, type Repo, type Port, type Status, type GitHubRepo } from '@/lib/api'
import { Logo } from '@/components/Logo'
import {
  ExternalLink,
  Copy,
  RefreshCw,
  Github,
  Server,
  HardDrive,
  Cpu,
  Lock,
  Unlock,
  KeyRound,
  FolderGit2,
  Plus,
  Search,
  Check,
  X,
  Command,
  Terminal,
  GitBranch,
  Share2,
  Timer,
  Trash2,
  Moon,
  Sun,
  Settings,
  HelpCircle,
  Activity,
  Clock,
  AlertCircle,
  MoreHorizontal,
} from 'lucide-react'

// Theme context
type Theme = 'light' | 'dark'
const ThemeContext = createContext<{ theme: Theme; toggleTheme: () => void }>({
  theme: 'light',
  toggleTheme: () => {},
})

// Toast notification system
type Toast = {
  id: number
  message: string
  type: 'success' | 'error' | 'info'
}

let toastId = 0

function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [repos, setRepos] = useState<Repo[]>([])
  const [ports, setPorts] = useState<Port[]>([])
  const [showCloneModal, setShowCloneModal] = useState(false)
  const [showCommandPalette, setShowCommandPalette] = useState(false)
  const [showHelpModal, setShowHelpModal] = useState(false)
  const [showSettingsModal, setShowSettingsModal] = useState(false)
  const [loading, setLoading] = useState(true)
  const [toasts, setToasts] = useState<Toast[]>([])
  const [theme, setTheme] = useState<Theme>(() => {
    if (typeof window !== 'undefined') {
      return (localStorage.getItem('theme') as Theme) || 'light'
    }
    return 'light'
  })
  const [portHealth, setPortHealth] = useState<Record<number, boolean>>({})
  const [error, setError] = useState<string | null>(null)

  const toggleTheme = useCallback(() => {
    setTheme(prev => {
      const next = prev === 'light' ? 'dark' : 'light'
      localStorage.setItem('theme', next)
      return next
    })
  }, [])

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
  }, [theme])

  const addToast = useCallback((message: string, type: Toast['type'] = 'success') => {
    const id = ++toastId
    setToasts(prev => [...prev, { id, message, type }])
    setTimeout(() => {
      setToasts(prev => prev.filter(t => t.id !== id))
    }, 3000)
  }, [])

  const fetchData = async () => {
    try {
      const [statusData, reposData, portsData] = await Promise.all([
        api.getStatus(),
        api.getRepos(),
        api.getPorts(),
      ])
      setStatus(statusData)
      setRepos(reposData)
      setPorts(portsData)
      setError(null)

      // Check port health
      const health: Record<number, boolean> = {}
      for (const port of portsData) {
        try {
          await fetch(`/${port.port}/`, { method: 'HEAD', mode: 'no-cors' })
          health[port.port] = true
        } catch {
          health[port.port] = false
        }
      }
      setPortHealth(health)
    } catch (err) {
      console.error('Failed to fetch data:', err)
      setError('Failed to connect to Homeport daemon')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't trigger if user is typing in an input
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return
      }

      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setShowCommandPalette(prev => !prev)
      }
      if (e.key === '?') {
        e.preventDefault()
        setShowHelpModal(prev => !prev)
      }
      if (e.key === 'Escape') {
        setShowCommandPalette(false)
        setShowCloneModal(false)
        setShowHelpModal(false)
        setShowSettingsModal(false)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const copyUrl = (port: number) => {
    const url = `${status?.config.external_url || window.location.origin}/${port}/`
    navigator.clipboard.writeText(url)
    addToast('URL copied to clipboard')
  }

  const copyCurl = (port: number) => {
    const url = `${status?.config.external_url || window.location.origin}/${port}/`
    const curl = `curl -X GET "${url}"`
    navigator.clipboard.writeText(curl)
    addToast('curl command copied')
  }

  const openPort = (port: number) => {
    window.open(`/${port}/`, '_blank')
  }

  const handleShare = async (port: number, mode: string, password?: string, expiresIn?: string) => {
    try {
      const result = await api.sharePort(port, mode, password, expiresIn)
      addToast(`Port ${port} shared as ${mode}${result.expires_at ? ' (expires ' + formatRelativeTime(result.expires_at) + ')' : ''}`)
      fetchData()
    } catch (err) {
      addToast('Failed to share port', 'error')
    }
  }

  const handleDeleteRepo = async (repo: Repo) => {
    if (!confirm(`Delete "${repo.name}"? This will remove the repository from disk.`)) {
      return
    }
    try {
      await api.deleteRepo(repo.id)
      addToast(`Deleted ${repo.name}`)
      fetchData()
    } catch (err) {
      addToast('Failed to delete repository', 'error')
    }
  }

  const handlePullAll = async () => {
    addToast('Pulling all repositories...', 'info')
    for (const repo of repos) {
      try {
        await api.pullRepo(repo.id)
      } catch (err) {
        console.error(`Failed to pull ${repo.name}:`, err)
      }
    }
    addToast('All repositories updated')
  }

  // Group ports by repo
  const portsByRepo = ports.reduce((acc, port) => {
    const key = port.repo_id || '_orphan'
    if (!acc[key]) acc[key] = []
    acc[key].push(port)
    return acc
  }, {} as Record<string, Port[]>)

  const orphanPorts = portsByRepo['_orphan'] || []
  delete portsByRepo['_orphan']

  if (loading) {
    return (
      <ThemeContext.Provider value={{ theme, toggleTheme }}>
        <div className={`min-h-screen ${theme === 'dark' ? 'dark bg-gray-950' : 'bg-gray-50'}`}>
          <LoadingSkeleton />
        </div>
      </ThemeContext.Provider>
    )
  }

  if (error) {
    return (
      <ThemeContext.Provider value={{ theme, toggleTheme }}>
        <div className={`min-h-screen flex items-center justify-center ${theme === 'dark' ? 'dark bg-gray-950' : 'bg-gray-50'}`}>
          <ErrorState message={error} onRetry={fetchData} />
        </div>
      </ThemeContext.Provider>
    )
  }

  return (
    <ThemeContext.Provider value={{ theme, toggleTheme }}>
      <div className={`min-h-screen transition-colors duration-200 ${theme === 'dark' ? 'dark bg-gray-950 text-gray-100' : 'bg-gray-50 text-gray-900'}`}>
        {/* Toast notifications */}
        <div className="fixed top-4 right-4 z-50 flex flex-col gap-2">
          {toasts.map(toast => (
            <div
              key={toast.id}
              className={`
                flex items-center gap-2 px-4 py-3 rounded-lg shadow-lg text-sm font-medium
                animate-in slide-in-from-right duration-200
                ${toast.type === 'success' ? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900' : ''}
                ${toast.type === 'error' ? 'bg-red-600 text-white' : ''}
                ${toast.type === 'info' ? 'bg-blue-600 text-white' : ''}
              `}
            >
              {toast.type === 'success' && <Check className="h-4 w-4" />}
              {toast.type === 'error' && <X className="h-4 w-4" />}
              {toast.type === 'info' && <Activity className="h-4 w-4" />}
              {toast.message}
            </div>
          ))}
        </div>

        {/* Header */}
        <header className={`sticky top-0 z-40 border-b transition-colors ${theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'}`}>
          <div className="max-w-6xl mx-auto px-4 sm:px-6 py-3 sm:py-4 flex items-center justify-between">
            <div className="flex items-center gap-2 sm:gap-3">
              <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${theme === 'dark' ? 'bg-white text-gray-900' : 'bg-gray-900 text-white'}`}>
                <Logo size={20} />
              </div>
              <h1 className="text-base sm:text-lg font-semibold">Homeport</h1>
              <Badge variant="outline" className="font-mono text-xs hidden sm:inline-flex">
                {status?.version}
              </Badge>
              <span className={`text-xs hidden md:inline-flex items-center gap-1 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
                <Clock className="h-3 w-3" />
                {status?.uptime}
              </span>
            </div>
            <div className="flex items-center gap-1 sm:gap-2">
              <button
                onClick={toggleTheme}
                className={`p-2 rounded-lg transition-colors ${theme === 'dark' ? 'hover:bg-gray-800 text-gray-400' : 'hover:bg-gray-100 text-gray-500'}`}
                title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              >
                {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
              </button>
              <button
                onClick={() => setShowHelpModal(true)}
                className={`p-2 rounded-lg transition-colors hidden sm:block ${theme === 'dark' ? 'hover:bg-gray-800 text-gray-400' : 'hover:bg-gray-100 text-gray-500'}`}
                title="Keyboard shortcuts (?)"
              >
                <HelpCircle className="h-4 w-4" />
              </button>
              <button
                onClick={() => setShowSettingsModal(true)}
                className={`p-2 rounded-lg transition-colors ${theme === 'dark' ? 'hover:bg-gray-800 text-gray-400' : 'hover:bg-gray-100 text-gray-500'}`}
                title="Settings"
              >
                <Settings className="h-4 w-4" />
              </button>
              <button
                onClick={() => setShowCommandPalette(true)}
                className={`items-center gap-2 px-3 py-1.5 text-sm rounded-lg transition-colors hidden sm:flex ${theme === 'dark' ? 'bg-gray-800 text-gray-400 hover:bg-gray-700' : 'bg-gray-100 text-gray-500 hover:bg-gray-200'}`}
              >
                <Command className="h-3 w-3" />
                <span>K</span>
              </button>
              <Button onClick={() => setShowCloneModal(true)} size="sm" className="hidden sm:flex">
                <Plus className="h-4 w-4 sm:mr-2" />
                <span className="hidden sm:inline">Clone Repo</span>
              </Button>
              <Button onClick={() => setShowCloneModal(true)} size="sm" className="sm:hidden p-2">
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </header>

        <main className="max-w-6xl mx-auto px-4 sm:px-6 py-6 sm:py-8 space-y-6 sm:space-y-8">
          {/* System Stats */}
          <div className="grid grid-cols-3 gap-2 sm:gap-4">
            <StatCard
              icon={<Cpu className="h-4 sm:h-5 w-4 sm:w-5" />}
              label="CPU"
              value={status?.stats.cpu_percent ?? 0}
              unit="%"
              theme={theme}
            />
            <StatCard
              icon={<Server className="h-4 sm:h-5 w-4 sm:w-5" />}
              label="Memory"
              value={status?.stats.memory_percent ?? 0}
              unit="%"
              detail={`${status?.stats.memory_used_gb?.toFixed(1) ?? 0}/${status?.stats.memory_total_gb?.toFixed(0) ?? 0}GB`}
              theme={theme}
            />
            <StatCard
              icon={<HardDrive className="h-4 sm:h-5 w-4 sm:w-5" />}
              label="Disk"
              value={status?.stats.disk_percent ?? 0}
              unit="%"
              detail={`${status?.stats.disk_used_gb?.toFixed(0) ?? 0}/${status?.stats.disk_total_gb?.toFixed(0) ?? 0}GB`}
              theme={theme}
            />
          </div>

          {/* Repos */}
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <h2 className={`text-sm sm:text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>
                Repositories
              </h2>
              <div className="flex items-center gap-2">
                <span className={`text-xs sm:text-sm ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
                  {repos.length} repos
                </span>
                {repos.length > 0 && (
                  <Button variant="ghost" size="sm" onClick={handlePullAll} className="hidden sm:flex">
                    <RefreshCw className="h-4 w-4 mr-1" />
                    Pull All
                  </Button>
                )}
              </div>
            </div>

            {repos.length === 0 ? (
              <EmptyState
                icon={<FolderGit2 className="h-12 w-12" />}
                title="No repositories yet"
                description="Clone a repository to get started with your development environment."
                action={
                  <Button variant="outline" onClick={() => setShowCloneModal(true)}>
                    <Plus className="h-4 w-4 mr-2" />
                    Clone your first repo
                  </Button>
                }
                theme={theme}
              />
            ) : (
              <div className="space-y-3">
                {repos.map((repo) => (
                  <RepoCard
                    key={repo.id}
                    repo={repo}
                    ports={portsByRepo[repo.id] || []}
                    portHealth={portHealth}
                    theme={theme}
                    onDelete={() => handleDeleteRepo(repo)}
                    onPull={() => {
                      api.pullRepo(repo.id)
                      addToast('Pulling latest changes...')
                    }}
                    onCopyUrl={copyUrl}
                    onCopyCurl={copyCurl}
                    onOpenPort={openPort}
                    onShare={handleShare}
                    addToast={addToast}
                  />
                ))}
              </div>
            )}
          </section>

          {/* Orphan Ports */}
          {orphanPorts.length > 0 && (
            <section className="space-y-4">
              <h2 className={`text-sm sm:text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>
                Other Ports
              </h2>
              <Card className={theme === 'dark' ? 'bg-gray-900 border-gray-800' : ''}>
                <CardContent className="pt-4 space-y-2">
                  {orphanPorts.map((port) => (
                    <PortRow
                      key={port.port}
                      port={port}
                      isHealthy={portHealth[port.port]}
                      theme={theme}
                      onCopy={() => copyUrl(port.port)}
                      onCopyCurl={() => copyCurl(port.port)}
                      onOpen={() => openPort(port.port)}
                      onShare={handleShare}
                    />
                  ))}
                </CardContent>
              </Card>
            </section>
          )}
        </main>

        {/* Modals */}
        {showCloneModal && (
          <CloneModal
            theme={theme}
            onClose={() => setShowCloneModal(false)}
            onClone={async (repo) => {
              await api.cloneRepo(repo)
              setShowCloneModal(false)
              addToast(`Cloned ${repo}`)
              fetchData()
            }}
            addToast={addToast}
          />
        )}

        {showCommandPalette && (
          <CommandPalette
            theme={theme}
            onClose={() => setShowCommandPalette(false)}
            repos={repos}
            ports={ports}
            onOpenClone={() => {
              setShowCommandPalette(false)
              setShowCloneModal(true)
            }}
            onOpenSettings={() => {
              setShowCommandPalette(false)
              setShowSettingsModal(true)
            }}
            onOpenPort={openPort}
            onCopyUrl={copyUrl}
            onToggleTheme={toggleTheme}
          />
        )}

        {showHelpModal && (
          <HelpModal theme={theme} onClose={() => setShowHelpModal(false)} />
        )}

        {showSettingsModal && (
          <SettingsModal
            theme={theme}
            status={status}
            onClose={() => setShowSettingsModal(false)}
          />
        )}
      </div>
    </ThemeContext.Provider>
  )
}

function LoadingSkeleton() {
  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 py-8 space-y-8 animate-pulse">
      {/* Header skeleton */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 bg-gray-200 dark:bg-gray-800 rounded-lg" />
          <div className="w-24 h-6 bg-gray-200 dark:bg-gray-800 rounded" />
        </div>
        <div className="w-32 h-9 bg-gray-200 dark:bg-gray-800 rounded" />
      </div>

      {/* Stats skeleton */}
      <div className="grid grid-cols-3 gap-4">
        {[1, 2, 3].map(i => (
          <div key={i} className="h-24 bg-gray-200 dark:bg-gray-800 rounded-lg" />
        ))}
      </div>

      {/* Repos skeleton */}
      <div className="space-y-3">
        {[1, 2].map(i => (
          <div key={i} className="h-32 bg-gray-200 dark:bg-gray-800 rounded-lg" />
        ))}
      </div>
    </div>
  )
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="text-center px-4">
      <div className="w-16 h-16 mx-auto mb-4 rounded-full bg-red-100 dark:bg-red-900/20 flex items-center justify-center">
        <AlertCircle className="h-8 w-8 text-red-600 dark:text-red-400" />
      </div>
      <h2 className="text-lg font-semibold mb-2 text-gray-900 dark:text-gray-100">Connection Error</h2>
      <p className="text-gray-500 dark:text-gray-400 mb-4">{message}</p>
      <Button onClick={onRetry}>
        <RefreshCw className="h-4 w-4 mr-2" />
        Retry
      </Button>
    </div>
  )
}

function EmptyState({
  icon,
  title,
  description,
  action,
  theme
}: {
  icon: React.ReactNode
  title: string
  description: string
  action?: React.ReactNode
  theme: Theme
}) {
  return (
    <Card className={`border-dashed ${theme === 'dark' ? 'bg-gray-900/50 border-gray-700' : ''}`}>
      <CardContent className="py-12 text-center">
        <div className={`mx-auto mb-4 ${theme === 'dark' ? 'text-gray-600' : 'text-gray-300'}`}>
          {icon}
        </div>
        <h3 className={`font-medium mb-1 ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>{title}</h3>
        <p className={`text-sm mb-4 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>{description}</p>
        {action}
      </CardContent>
    </Card>
  )
}

function StatCard({
  icon,
  label,
  value,
  unit,
  detail,
  theme
}: {
  icon: React.ReactNode
  label: string
  value: number
  unit: string
  detail?: string
  theme: Theme
}) {
  const percentage = Math.min(100, Math.max(0, value))
  const color = percentage > 80 ? 'bg-red-500' : percentage > 60 ? 'bg-yellow-500' : 'bg-green-500'

  return (
    <Card className={`group transition-all duration-200 hover:shadow-md ${theme === 'dark' ? 'bg-gray-900 border-gray-800 hover:border-gray-700' : 'hover:shadow-lg'}`}>
      <CardContent className="pt-4 sm:pt-5 pb-3 sm:pb-4 px-3 sm:px-6">
        <div className="flex items-start justify-between mb-2 sm:mb-3">
          <div className={`p-1.5 sm:p-2 rounded-lg transition-colors ${theme === 'dark' ? 'bg-gray-800 text-gray-400 group-hover:bg-gray-700' : 'bg-gray-100 text-gray-600 group-hover:bg-gray-200'}`}>
            {icon}
          </div>
          <div className="text-right">
            <span className="text-xl sm:text-2xl font-semibold">{value.toFixed(0)}</span>
            <span className={`text-xs sm:text-sm ml-0.5 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>{unit}</span>
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex justify-between text-xs sm:text-sm">
            <span className={theme === 'dark' ? 'text-gray-400' : 'text-gray-600'}>{label}</span>
            {detail && <span className={`hidden sm:inline ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>{detail}</span>}
          </div>
          <div className={`h-1 sm:h-1.5 rounded-full overflow-hidden ${theme === 'dark' ? 'bg-gray-800' : 'bg-gray-100'}`}>
            <div className={`h-full ${color} transition-all duration-500`} style={{ width: `${percentage}%` }} />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function RepoCard({
  repo,
  ports,
  portHealth,
  theme,
  onDelete,
  onPull,
  onCopyUrl,
  onCopyCurl,
  onOpenPort,
  onShare,
}: {
  repo: Repo
  ports: Port[]
  portHealth: Record<number, boolean>
  theme: Theme
  onDelete: () => void
  onPull: () => void
  onCopyUrl: (port: number) => void
  onCopyCurl: (port: number) => void
  onOpenPort: (port: number) => void
  onShare: (port: number, mode: string, password?: string, expiresIn?: string) => void
  addToast?: (msg: string, type?: 'success' | 'error' | 'info') => void
}) {
  const [showMenu, setShowMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <Card className={`overflow-hidden transition-all duration-200 hover:shadow-md ${theme === 'dark' ? 'bg-gray-900 border-gray-800 hover:border-gray-700' : 'hover:shadow-lg'}`}>
      <CardHeader className={`pb-3 ${theme === 'dark' ? 'bg-gray-800/50' : 'bg-gray-50/50'}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            <div className={`w-7 h-7 sm:w-8 sm:h-8 rounded-lg flex items-center justify-center flex-shrink-0 ${theme === 'dark' ? 'bg-gray-700' : 'bg-gray-100'}`}>
              <GitBranch className={`h-3.5 w-3.5 sm:h-4 sm:w-4 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-600'}`} />
            </div>
            <div className="min-w-0">
              <CardTitle className="text-sm sm:text-base truncate">{repo.name}</CardTitle>
              <p className={`text-xs font-mono truncate ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>{repo.path}</p>
            </div>
            {repo.github_url && (
              <a
                href={repo.github_url}
                target="_blank"
                rel="noopener noreferrer"
                className={`transition-colors flex-shrink-0 ${theme === 'dark' ? 'text-gray-500 hover:text-gray-300' : 'text-gray-400 hover:text-gray-600'}`}
              >
                <Github className="h-4 w-4" />
              </a>
            )}
          </div>
          <div className="flex items-center gap-1 sm:gap-2">
            <Button variant="ghost" size="sm" onClick={onPull} className="hidden sm:flex">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <a href={`/code/?folder=${repo.path}`} target="_blank" rel="noopener noreferrer" className="hidden sm:block">
              <Button variant="outline" size="sm">
                Open in VS Code
              </Button>
            </a>
            <div className="relative" ref={menuRef}>
              <Button variant="ghost" size="sm" onClick={() => setShowMenu(!showMenu)} className="p-2">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
              {showMenu && (
                <div className={`absolute right-0 top-full mt-1 w-48 rounded-lg shadow-lg border z-50 py-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
                  <a
                    href={`/code/?folder=${repo.path}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm sm:hidden ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Terminal className="h-4 w-4" />
                    Open in VS Code
                  </a>
                  <button
                    onClick={() => { onPull(); setShowMenu(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm sm:hidden ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <RefreshCw className="h-4 w-4" />
                    Pull Latest
                  </button>
                  <button
                    onClick={() => { onDelete(); setShowMenu(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm text-red-600 ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete Repository
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className="pt-3">
        {ports.length > 0 ? (
          <div className="space-y-2">
            {ports.map((port) => (
              <PortRow
                key={port.port}
                port={port}
                isHealthy={portHealth[port.port]}
                theme={theme}
                onCopy={() => onCopyUrl(port.port)}
                onCopyCurl={() => onCopyCurl(port.port)}
                onOpen={() => onOpenPort(port.port)}
                onShare={onShare}
              />
            ))}
          </div>
        ) : (
          <p className={`text-sm py-2 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>No dev servers running</p>
        )}
      </CardContent>
    </Card>
  )
}

function PortRow({
  port,
  isHealthy,
  theme,
  onCopy,
  onCopyCurl,
  onOpen,
  onShare,
}: {
  port: Port
  isHealthy?: boolean
  theme: Theme
  onCopy: () => void
  onCopyCurl: () => void
  onOpen: () => void
  onShare: (port: number, mode: string, password?: string, expiresIn?: string) => void
}) {
  const [showShareMenu, setShowShareMenu] = useState(false)
  const [showCopyMenu, setShowCopyMenu] = useState(false)
  const shareMenuRef = useRef<HTMLDivElement>(null)
  const copyMenuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (shareMenuRef.current && !shareMenuRef.current.contains(e.target as Node)) {
        setShowShareMenu(false)
      }
      if (copyMenuRef.current && !copyMenuRef.current.contains(e.target as Node)) {
        setShowCopyMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const ShareIcon = port.share_mode === 'public' ? Unlock :
                    port.share_mode === 'password' ? KeyRound : Lock

  const modeColors = {
    private: theme === 'dark' ? 'bg-gray-800 text-gray-400' : 'bg-gray-100 text-gray-700',
    password: theme === 'dark' ? 'bg-amber-900/30 text-amber-400' : 'bg-amber-50 text-amber-700',
    public: theme === 'dark' ? 'bg-green-900/30 text-green-400' : 'bg-green-50 text-green-700'
  }

  return (
    <div className={`flex items-center justify-between py-2 sm:py-2.5 px-2 sm:px-3 rounded-lg transition-colors group ${theme === 'dark' ? 'bg-gray-800/50 hover:bg-gray-800' : 'bg-gray-50 hover:bg-gray-100'}`}>
      <div className="flex items-center gap-2 sm:gap-3 min-w-0">
        {/* Health indicator */}
        <div className={`w-2 h-2 rounded-full flex-shrink-0 ${isHealthy === true ? 'bg-green-500' : isHealthy === false ? 'bg-red-500' : 'bg-gray-400'}`} title={isHealthy ? 'Responding' : 'Not responding'} />

        <code className={`text-xs sm:text-sm font-mono font-medium px-1.5 sm:px-2 py-0.5 sm:py-1 rounded border flex-shrink-0 ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-200' : 'bg-white border-gray-200 text-gray-900'}`}>
          :{port.port}
        </code>
        <span className={`text-xs sm:text-sm truncate ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
          {port.process_name || 'Unknown'}
        </span>
        <div className={`hidden sm:flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${modeColors[port.share_mode]}`}>
          <ShareIcon className="h-3 w-3" />
          {port.share_mode}
          {port.expires_at && (
            <span className="flex items-center gap-0.5 ml-1 opacity-75">
              <Timer className="h-3 w-3" />
              {formatRelativeTime(port.expires_at)}
            </span>
          )}
        </div>
        {/* Last seen */}
        <span className={`hidden md:inline text-xs ${theme === 'dark' ? 'text-gray-600' : 'text-gray-400'}`}>
          seen {formatRelativeTime(port.last_seen)} ago
        </span>
      </div>
      <div className="flex items-center gap-0.5 sm:gap-1 opacity-100 sm:opacity-0 group-hover:opacity-100 transition-opacity">
        {/* Copy menu */}
        <div className="relative" ref={copyMenuRef}>
          <Button variant="ghost" size="sm" onClick={() => setShowCopyMenu(!showCopyMenu)} className="h-7 w-7 sm:h-8 sm:w-8 p-0">
            <Copy className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
          </Button>
          {showCopyMenu && (
            <div className={`absolute right-0 top-full mt-1 w-36 rounded-lg shadow-lg border z-50 py-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
              <button
                onClick={() => { onCopy(); setShowCopyMenu(false); }}
                className={`flex items-center gap-2 w-full px-3 py-2 text-sm ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
              >
                Copy URL
              </button>
              <button
                onClick={() => { onCopyCurl(); setShowCopyMenu(false); }}
                className={`flex items-center gap-2 w-full px-3 py-2 text-sm ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
              >
                Copy as curl
              </button>
            </div>
          )}
        </div>
        <Button variant="ghost" size="sm" onClick={onOpen} className="h-7 w-7 sm:h-8 sm:w-8 p-0">
          <ExternalLink className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
        </Button>
        <div className="relative" ref={shareMenuRef}>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowShareMenu(!showShareMenu)}
            className="h-7 w-7 sm:h-8 sm:w-8 p-0"
          >
            <Share2 className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
          </Button>
          {showShareMenu && (
            <ShareMenu
              port={port}
              theme={theme}
              onShare={(mode, password, expiresIn) => {
                onShare(port.port, mode, password, expiresIn)
                setShowShareMenu(false)
              }}
              onClose={() => setShowShareMenu(false)}
            />
          )}
        </div>
      </div>
    </div>
  )
}

function ShareMenu({
  port,
  theme,
  onShare,
  onClose
}: {
  port: Port
  theme: Theme
  onShare: (mode: string, password?: string, expiresIn?: string) => void
  onClose: () => void
}) {
  const [mode, setMode] = useState(port.share_mode)
  const [password, setPassword] = useState('')
  const [expiresIn, setExpiresIn] = useState('')

  return (
    <div className={`absolute right-0 top-full mt-1 w-64 rounded-lg shadow-lg border p-3 z-50 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
      <div className="space-y-3">
        <div>
          <label className={`text-xs font-medium mb-1.5 block ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Sharing Mode</label>
          <div className="flex gap-1">
            {(['private', 'password', 'public'] as const).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`
                  flex-1 px-2 py-1.5 text-xs font-medium rounded-md capitalize transition-colors
                  ${mode === m
                    ? theme === 'dark' ? 'bg-white text-gray-900' : 'bg-gray-900 text-white'
                    : theme === 'dark' ? 'bg-gray-700 text-gray-300 hover:bg-gray-600' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}
                `}
              >
                {m}
              </button>
            ))}
          </div>
        </div>

        {mode === 'password' && (
          <div>
            <label className={`text-xs font-medium mb-1.5 block ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter password..."
              className={`w-full px-2.5 py-1.5 text-sm border rounded-md focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-900 border-gray-600 text-gray-100 focus:ring-gray-500' : 'border-gray-200 focus:ring-gray-900'}`}
            />
          </div>
        )}

        {mode !== 'private' && (
          <div>
            <label className={`text-xs font-medium mb-1.5 block ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Expires</label>
            <select
              value={expiresIn}
              onChange={(e) => setExpiresIn(e.target.value)}
              className={`w-full px-2.5 py-1.5 text-sm border rounded-md focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-900 border-gray-600 text-gray-100 focus:ring-gray-500' : 'border-gray-200 focus:ring-gray-900 bg-white'}`}
            >
              <option value="">Never</option>
              <option value="1h">1 hour</option>
              <option value="24h">24 hours</option>
              <option value="7d">7 days</option>
              <option value="30d">30 days</option>
            </select>
          </div>
        )}

        <div className="flex gap-2 pt-1">
          <Button variant="outline" size="sm" onClick={onClose} className="flex-1">
            Cancel
          </Button>
          <Button
            size="sm"
            onClick={() => onShare(mode, mode === 'password' ? password : undefined, expiresIn || undefined)}
            className="flex-1"
            disabled={mode === 'password' && !password}
          >
            Apply
          </Button>
        </div>
      </div>
    </div>
  )
}

function CloneModal({
  theme,
  onClose,
  onClone,
  addToast
}: {
  theme: Theme
  onClose: () => void
  onClone: (repo: string) => Promise<void>
  addToast: (msg: string, type?: 'success' | 'error' | 'info') => void
}) {
  const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([])
  const [filteredRepos, setFilteredRepos] = useState<GitHubRepo[]>([])
  const [loading, setLoading] = useState(true)
  const [cloning, setCloning] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  const [searching, setSearching] = useState(false)
  const searchInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    api.getGitHubRepos(50).then(repos => {
      setGithubRepos(repos)
      setFilteredRepos(repos)
    }).finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    searchInputRef.current?.focus()
  }, [])

  useEffect(() => {
    const query = searchQuery.toLowerCase().trim()
    if (!query) {
      setFilteredRepos(githubRepos)
      return
    }

    const localMatches = githubRepos.filter(
      r => r.name.toLowerCase().includes(query) ||
           r.nameWithOwner.toLowerCase().includes(query) ||
           r.description?.toLowerCase().includes(query)
    )
    setFilteredRepos(localMatches)

    if (query.length >= 3) {
      setSearching(true)
      const timeout = setTimeout(() => {
        api.searchGitHubRepos(query).then(results => {
          const existing = new Set(localMatches.map(r => r.nameWithOwner))
          const newResults = results.filter(r => !existing.has(r.nameWithOwner))
          setFilteredRepos([...localMatches, ...newResults])
        }).finally(() => setSearching(false))
      }, 300)
      return () => clearTimeout(timeout)
    }
  }, [searchQuery, githubRepos])

  const handleClone = async (repo: string) => {
    setCloning(repo)
    try {
      await onClone(repo)
    } catch (err) {
      addToast('Failed to clone repository', 'error')
    } finally {
      setCloning(null)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-xl overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <div className="relative">
            <Search className={`absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
            <input
              ref={searchInputRef}
              type="text"
              placeholder="Search repositories..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className={`w-full pl-10 pr-4 py-2.5 text-sm border rounded-lg focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-100 focus:ring-gray-600' : 'border-gray-200 focus:ring-gray-900'}`}
            />
            {searching && (
              <RefreshCw className={`absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 animate-spin ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
            )}
          </div>
        </div>
        <div className="max-h-[50vh] overflow-y-auto">
          {loading ? (
            <div className="flex justify-center py-12">
              <RefreshCw className={`h-6 w-6 animate-spin ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
            </div>
          ) : filteredRepos.length === 0 ? (
            <div className={`py-12 text-center ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
              No repositories found
            </div>
          ) : (
            <div className={`divide-y ${theme === 'dark' ? 'divide-gray-800' : 'divide-gray-100'}`}>
              {filteredRepos.map((repo) => (
                <div
                  key={repo.nameWithOwner}
                  className={`flex items-center justify-between p-4 transition-colors ${theme === 'dark' ? 'hover:bg-gray-800' : 'hover:bg-gray-50'}`}
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className={`font-medium truncate ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>{repo.name}</span>
                      {repo.isPrivate && (
                        <Lock className={`h-3 w-3 flex-shrink-0 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
                      )}
                    </div>
                    <p className={`text-xs truncate ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>{repo.nameWithOwner}</p>
                    {repo.description && (
                      <p className={`text-sm mt-1 line-clamp-1 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{repo.description}</p>
                    )}
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => handleClone(repo.nameWithOwner)}
                    disabled={cloning !== null}
                    className="ml-4 flex-shrink-0"
                  >
                    {cloning === repo.nameWithOwner ? (
                      <RefreshCw className="h-4 w-4 animate-spin" />
                    ) : (
                      'Clone'
                    )}
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
        <div className={`p-3 border-t ${theme === 'dark' ? 'border-gray-800 bg-gray-800/50' : 'border-gray-100 bg-gray-50'}`}>
          <p className={`text-xs text-center ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
            Press <kbd className={`px-1.5 py-0.5 rounded ${theme === 'dark' ? 'bg-gray-700 border-gray-600 text-gray-300' : 'bg-white border border-gray-200 text-gray-600'}`}>Esc</kbd> to close
          </p>
        </div>
      </div>
    </div>
  )
}

function CommandPalette({
  theme,
  onClose,
  repos,
  ports,
  onOpenClone,
  onOpenSettings,
  onOpenPort,
  onCopyUrl,
  onToggleTheme
}: {
  theme: Theme
  onClose: () => void
  repos: Repo[]
  ports: Port[]
  onOpenClone: () => void
  onOpenSettings: () => void
  onOpenPort: (port: number) => void
  onCopyUrl: (port: number) => void
  onToggleTheme: () => void
}) {
  const [query, setQuery] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)

  type Command = {
    id: string
    label: string
    icon: React.ReactNode
    action: () => void
    category: string
  }

  const commands: Command[] = [
    {
      id: 'clone',
      label: 'Clone repository',
      icon: <Plus className="h-4 w-4" />,
      action: onOpenClone,
      category: 'Actions'
    },
    {
      id: 'theme',
      label: `Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`,
      icon: theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />,
      action: onToggleTheme,
      category: 'Actions'
    },
    {
      id: 'settings',
      label: 'Open settings',
      icon: <Settings className="h-4 w-4" />,
      action: onOpenSettings,
      category: 'Actions'
    },
    ...ports.map(p => ({
      id: `port-${p.port}`,
      label: `Open :${p.port}`,
      icon: <ExternalLink className="h-4 w-4" />,
      action: () => onOpenPort(p.port),
      category: 'Ports'
    })),
    ...ports.map(p => ({
      id: `copy-${p.port}`,
      label: `Copy URL for :${p.port}`,
      icon: <Copy className="h-4 w-4" />,
      action: () => onCopyUrl(p.port),
      category: 'Ports'
    })),
    ...repos.map(r => ({
      id: `code-${r.id}`,
      label: `Open ${r.name} in VS Code`,
      icon: <Terminal className="h-4 w-4" />,
      action: () => window.open(`/code/?folder=${r.path}`, '_blank'),
      category: 'Repositories'
    })),
  ]

  const filteredCommands = query
    ? commands.filter(c => c.label.toLowerCase().includes(query.toLowerCase()))
    : commands

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  useEffect(() => {
    setSelectedIndex(0)
  }, [query])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelectedIndex(i => Math.min(i + 1, filteredCommands.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelectedIndex(i => Math.max(i - 1, 0))
    } else if (e.key === 'Enter' && filteredCommands[selectedIndex]) {
      filteredCommands[selectedIndex].action()
      onClose()
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[15vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-lg overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`p-3 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <div className="relative">
            <Search className={`absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
            <input
              ref={inputRef}
              type="text"
              placeholder="Type a command..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              className={`w-full pl-10 pr-4 py-2 text-sm border-0 focus:outline-none focus:ring-0 ${theme === 'dark' ? 'bg-gray-900 text-gray-100' : ''}`}
            />
          </div>
        </div>
        <div className="max-h-[40vh] overflow-y-auto py-2">
          {filteredCommands.length === 0 ? (
            <div className={`py-8 text-center text-sm ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
              No commands found
            </div>
          ) : (
            filteredCommands.map((cmd, index) => (
              <button
                key={cmd.id}
                onClick={() => {
                  cmd.action()
                  onClose()
                }}
                className={`
                  w-full flex items-center gap-3 px-4 py-2.5 text-sm text-left transition-colors
                  ${index === selectedIndex ? theme === 'dark' ? 'bg-gray-800' : 'bg-gray-100' : theme === 'dark' ? 'hover:bg-gray-800' : 'hover:bg-gray-50'}
                `}
              >
                <span className={theme === 'dark' ? 'text-gray-400' : 'text-gray-400'}>{cmd.icon}</span>
                <span className="flex-1">{cmd.label}</span>
                <span className={`text-xs ${theme === 'dark' ? 'text-gray-600' : 'text-gray-400'}`}>{cmd.category}</span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

function HelpModal({ theme, onClose }: { theme: Theme; onClose: () => void }) {
  const shortcuts = [
    { keys: ['Cmd', 'K'], description: 'Open command palette' },
    { keys: ['?'], description: 'Show keyboard shortcuts' },
    { keys: ['Esc'], description: 'Close modal / cancel' },
  ]

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`px-6 py-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <h2 className={`text-lg font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Keyboard Shortcuts</h2>
        </div>
        <div className="px-6 py-4 space-y-3">
          {shortcuts.map((shortcut, i) => (
            <div key={i} className="flex items-center justify-between">
              <span className={theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}>{shortcut.description}</span>
              <div className="flex items-center gap-1">
                {shortcut.keys.map((key, j) => (
                  <kbd
                    key={j}
                    className={`px-2 py-1 text-xs font-mono rounded ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-300' : 'bg-gray-100 border border-gray-200 text-gray-600'}`}
                  >
                    {key}
                  </kbd>
                ))}
              </div>
            </div>
          ))}
        </div>
        <div className={`px-6 py-3 border-t ${theme === 'dark' ? 'border-gray-800 bg-gray-800/50' : 'border-gray-100 bg-gray-50'}`}>
          <Button variant="outline" onClick={onClose} className="w-full">
            Close
          </Button>
        </div>
      </div>
    </div>
  )
}

function SettingsModal({
  theme,
  status,
  onClose
}: {
  theme: Theme
  status: Status | null
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`px-6 py-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <h2 className={`text-lg font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Settings</h2>
        </div>
        <div className="px-6 py-4 space-y-4">
          <div>
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>External URL</label>
            <p className={`text-sm mt-1 font-mono ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
              {status?.config.external_url || 'Not set'}
            </p>
          </div>
          <div>
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>Port Range</label>
            <p className={`text-sm mt-1 font-mono ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
              {status?.config.port_range || 'Not set'}
            </p>
          </div>
          <div>
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>Mode</label>
            <p className={`text-sm mt-1 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
              {status?.config.dev_mode ? 'Development' : 'Production'}
            </p>
          </div>
          <div>
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>Version</label>
            <p className={`text-sm mt-1 font-mono ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
              {status?.version}
            </p>
          </div>
        </div>
        <div className={`px-6 py-3 border-t ${theme === 'dark' ? 'border-gray-800 bg-gray-800/50' : 'border-gray-100 bg-gray-50'}`}>
          <p className={`text-xs text-center mb-3 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
            Settings are configured via environment variables and config files.
          </p>
          <Button variant="outline" onClick={onClose} className="w-full">
            Close
          </Button>
        </div>
      </div>
    </div>
  )
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diff = date.getTime() - now.getTime()
  const absDiff = Math.abs(diff)

  const minutes = Math.floor(absDiff / (1000 * 60))
  const hours = Math.floor(absDiff / (1000 * 60 * 60))
  const days = Math.floor(hours / 24)

  if (diff < 0) {
    // Past
    if (days > 0) return `${days}d`
    if (hours > 0) return `${hours}h`
    if (minutes > 0) return `${minutes}m`
    return 'now'
  } else {
    // Future (expires)
    if (days > 0) return `${days}d`
    if (hours > 0) return `${hours}h`
    if (minutes > 0) return `${minutes}m`
    return 'soon'
  }
}

export default App
