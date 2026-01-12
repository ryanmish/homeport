import { useEffect, useState, useCallback, useRef } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { api, type Repo, type Port, type Status, type GitHubRepo } from '@/lib/api'
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
} from 'lucide-react'

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
  const [loading, setLoading] = useState(true)
  const [toasts, setToasts] = useState<Toast[]>([])

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
    } catch (err) {
      console.error('Failed to fetch data:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 5000)
    return () => clearInterval(interval)
  }, [])

  // Cmd+K handler
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setShowCommandPalette(prev => !prev)
      }
      if (e.key === 'Escape') {
        setShowCommandPalette(false)
        setShowCloneModal(false)
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
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 animate-spin text-gray-400" />
          <span className="text-sm text-gray-500">Loading...</span>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Toast notifications */}
      <div className="fixed top-4 right-4 z-50 flex flex-col gap-2">
        {toasts.map(toast => (
          <div
            key={toast.id}
            className={`
              flex items-center gap-2 px-4 py-3 rounded-lg shadow-lg text-sm font-medium
              animate-in slide-in-from-right duration-200
              ${toast.type === 'success' ? 'bg-gray-900 text-white' : ''}
              ${toast.type === 'error' ? 'bg-red-600 text-white' : ''}
              ${toast.type === 'info' ? 'bg-blue-600 text-white' : ''}
            `}
          >
            {toast.type === 'success' && <Check className="h-4 w-4" />}
            {toast.type === 'error' && <X className="h-4 w-4" />}
            {toast.message}
          </div>
        ))}
      </div>

      {/* Header */}
      <header className="bg-white border-b border-gray-200 sticky top-0 z-40">
        <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 bg-gray-900 rounded-lg flex items-center justify-center">
              <Terminal className="h-4 w-4 text-white" />
            </div>
            <h1 className="text-lg font-semibold text-gray-900">Homeport</h1>
            <Badge variant="outline" className="font-mono text-xs">
              {status?.version}
            </Badge>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => setShowCommandPalette(true)}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-gray-500 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
            >
              <Command className="h-3 w-3" />
              <span>K</span>
            </button>
            <Button onClick={() => setShowCloneModal(true)} size="sm">
              <Plus className="h-4 w-4 mr-2" />
              Clone Repo
            </Button>
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto px-6 py-8 space-y-8">
        {/* System Stats */}
        <div className="grid grid-cols-3 gap-4">
          <StatCard
            icon={<Cpu className="h-5 w-5" />}
            label="CPU"
            value={status?.stats.cpu_percent ?? 0}
            unit="%"
          />
          <StatCard
            icon={<Server className="h-5 w-5" />}
            label="Memory"
            value={status?.stats.memory_percent ?? 0}
            unit="%"
            detail={`${status?.stats.memory_used_gb?.toFixed(1) ?? 0} / ${status?.stats.memory_total_gb?.toFixed(1) ?? 0} GB`}
          />
          <StatCard
            icon={<HardDrive className="h-5 w-5" />}
            label="Disk"
            value={status?.stats.disk_percent ?? 0}
            unit="%"
            detail={`${status?.stats.disk_used_gb?.toFixed(0) ?? 0} / ${status?.stats.disk_total_gb?.toFixed(0) ?? 0} GB`}
          />
        </div>

        {/* Repos */}
        <section className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-gray-900">Repositories</h2>
            <span className="text-sm text-gray-500">{repos.length} repos</span>
          </div>

          {repos.length === 0 ? (
            <Card className="border-dashed">
              <CardContent className="py-12 text-center">
                <FolderGit2 className="h-12 w-12 mx-auto mb-4 text-gray-300" />
                <p className="text-gray-500 mb-4">No repositories yet</p>
                <Button variant="outline" onClick={() => setShowCloneModal(true)}>
                  <Plus className="h-4 w-4 mr-2" />
                  Clone your first repo
                </Button>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-3">
              {repos.map((repo) => (
                <Card key={repo.id} className="overflow-hidden hover:shadow-md transition-shadow">
                  <CardHeader className="pb-3 bg-gray-50/50">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 bg-gray-100 rounded-lg flex items-center justify-center">
                          <GitBranch className="h-4 w-4 text-gray-600" />
                        </div>
                        <div>
                          <CardTitle className="text-base">{repo.name}</CardTitle>
                          <p className="text-xs text-gray-500 font-mono">{repo.path}</p>
                        </div>
                        {repo.github_url && (
                          <a
                            href={repo.github_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-gray-400 hover:text-gray-600 transition-colors"
                          >
                            <Github className="h-4 w-4" />
                          </a>
                        )}
                      </div>
                      <div className="flex items-center gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            api.pullRepo(repo.id)
                            addToast('Pulling latest changes...')
                          }}
                        >
                          <RefreshCw className="h-4 w-4" />
                        </Button>
                        <a
                          href={`/code/?folder=${repo.path}`}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          <Button variant="outline" size="sm">
                            Open in VS Code
                          </Button>
                        </a>
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="pt-3">
                    {portsByRepo[repo.id]?.length > 0 ? (
                      <div className="space-y-2">
                        {portsByRepo[repo.id].map((port) => (
                          <PortRow
                            key={port.port}
                            port={port}
                            onCopy={() => copyUrl(port.port)}
                            onOpen={() => openPort(port.port)}
                            onShare={handleShare}
                            addToast={addToast}
                          />
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-gray-400 py-2">No dev servers running</p>
                    )}
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </section>

        {/* Orphan Ports */}
        {orphanPorts.length > 0 && (
          <section className="space-y-4">
            <h2 className="text-base font-semibold text-gray-900">Other Ports</h2>
            <Card>
              <CardContent className="pt-4 space-y-2">
                {orphanPorts.map((port) => (
                  <PortRow
                    key={port.port}
                    port={port}
                    onCopy={() => copyUrl(port.port)}
                    onOpen={() => openPort(port.port)}
                    onShare={handleShare}
                    addToast={addToast}
                  />
                ))}
              </CardContent>
            </Card>
          </section>
        )}
      </main>

      {/* Clone Modal */}
      {showCloneModal && (
        <CloneModal
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

      {/* Command Palette */}
      {showCommandPalette && (
        <CommandPalette
          onClose={() => setShowCommandPalette(false)}
          repos={repos}
          ports={ports}
          onOpenClone={() => {
            setShowCommandPalette(false)
            setShowCloneModal(true)
          }}
          onOpenPort={openPort}
          onCopyUrl={copyUrl}
        />
      )}
    </div>
  )
}

function StatCard({
  icon,
  label,
  value,
  unit,
  detail
}: {
  icon: React.ReactNode
  label: string
  value: number
  unit: string
  detail?: string
}) {
  const percentage = Math.min(100, Math.max(0, value))
  const color = percentage > 80 ? 'bg-red-500' : percentage > 60 ? 'bg-yellow-500' : 'bg-green-500'

  return (
    <Card>
      <CardContent className="pt-5">
        <div className="flex items-start justify-between mb-3">
          <div className="p-2 bg-gray-100 rounded-lg text-gray-600">
            {icon}
          </div>
          <div className="text-right">
            <span className="text-2xl font-semibold">{value.toFixed(1)}</span>
            <span className="text-gray-500 text-sm ml-0.5">{unit}</span>
          </div>
        </div>
        <div className="space-y-1">
          <div className="flex justify-between text-sm">
            <span className="text-gray-600">{label}</span>
            {detail && <span className="text-gray-400">{detail}</span>}
          </div>
          <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
            <div className={`h-full ${color} transition-all duration-500`} style={{ width: `${percentage}%` }} />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function PortRow({
  port,
  onCopy,
  onOpen,
  onShare,
}: {
  port: Port
  onCopy: () => void
  onOpen: () => void
  onShare: (port: number, mode: string, password?: string, expiresIn?: string) => void
  addToast?: (msg: string, type?: 'success' | 'error' | 'info') => void
}) {
  const [showShareMenu, setShowShareMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowShareMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const ShareIcon = port.share_mode === 'public' ? Unlock :
                    port.share_mode === 'password' ? KeyRound : Lock

  const modeColors = {
    private: 'bg-gray-100 text-gray-700',
    password: 'bg-amber-50 text-amber-700',
    public: 'bg-green-50 text-green-700'
  }

  return (
    <div className="flex items-center justify-between py-2.5 px-3 rounded-lg bg-gray-50 hover:bg-gray-100 transition-colors group">
      <div className="flex items-center gap-3">
        <code className="text-sm font-mono font-medium text-gray-900 bg-white px-2 py-1 rounded border border-gray-200">
          :{port.port}
        </code>
        <span className="text-sm text-gray-500">
          {port.process_name || 'Unknown process'}
        </span>
        <div className={`flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${modeColors[port.share_mode]}`}>
          <ShareIcon className="h-3 w-3" />
          {port.share_mode}
          {port.expires_at && (
            <span className="flex items-center gap-0.5 ml-1 opacity-75">
              <Timer className="h-3 w-3" />
              {formatRelativeTime(port.expires_at)}
            </span>
          )}
        </div>
      </div>
      <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
        <Button variant="ghost" size="sm" onClick={onCopy} className="h-8 w-8 p-0">
          <Copy className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="sm" onClick={onOpen} className="h-8 w-8 p-0">
          <ExternalLink className="h-4 w-4" />
        </Button>
        <div className="relative" ref={menuRef}>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowShareMenu(!showShareMenu)}
            className="h-8 w-8 p-0"
          >
            <Share2 className="h-4 w-4" />
          </Button>
          {showShareMenu && (
            <ShareMenu
              port={port}
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
  onShare,
  onClose
}: {
  port: Port
  onShare: (mode: string, password?: string, expiresIn?: string) => void
  onClose: () => void
}) {
  const [mode, setMode] = useState(port.share_mode)
  const [password, setPassword] = useState('')
  const [expiresIn, setExpiresIn] = useState('')

  return (
    <div className="absolute right-0 top-full mt-1 w-64 bg-white rounded-lg shadow-lg border border-gray-200 p-3 z-50">
      <div className="space-y-3">
        <div>
          <label className="text-xs font-medium text-gray-500 mb-1.5 block">Sharing Mode</label>
          <div className="flex gap-1">
            {(['private', 'password', 'public'] as const).map((m) => (
              <button
                key={m}
                onClick={() => setMode(m)}
                className={`
                  flex-1 px-2 py-1.5 text-xs font-medium rounded-md capitalize transition-colors
                  ${mode === m
                    ? 'bg-gray-900 text-white'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}
                `}
              >
                {m}
              </button>
            ))}
          </div>
        </div>

        {mode === 'password' && (
          <div>
            <label className="text-xs font-medium text-gray-500 mb-1.5 block">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter password..."
              className="w-full px-2.5 py-1.5 text-sm border border-gray-200 rounded-md focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"
            />
          </div>
        )}

        {mode !== 'private' && (
          <div>
            <label className="text-xs font-medium text-gray-500 mb-1.5 block">Expires</label>
            <select
              value={expiresIn}
              onChange={(e) => setExpiresIn(e.target.value)}
              className="w-full px-2.5 py-1.5 text-sm border border-gray-200 rounded-md focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent bg-white"
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
  onClose,
  onClone,
  addToast
}: {
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

    // Filter local results first
    const localMatches = githubRepos.filter(
      r => r.name.toLowerCase().includes(query) ||
           r.nameWithOwner.toLowerCase().includes(query) ||
           r.description?.toLowerCase().includes(query)
    )
    setFilteredRepos(localMatches)

    // If query looks like a search, hit the API
    if (query.length >= 3) {
      setSearching(true)
      const timeout = setTimeout(() => {
        api.searchGitHubRepos(query).then(results => {
          // Merge with local results, avoiding duplicates
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
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-2xl w-full max-w-xl overflow-hidden animate-in fade-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-4 border-b border-gray-100">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              ref={searchInputRef}
              type="text"
              placeholder="Search repositories..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-10 pr-4 py-2.5 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"
            />
            {searching && (
              <RefreshCw className="absolute right-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400 animate-spin" />
            )}
          </div>
        </div>
        <div className="max-h-[50vh] overflow-y-auto">
          {loading ? (
            <div className="flex justify-center py-12">
              <RefreshCw className="h-6 w-6 animate-spin text-gray-400" />
            </div>
          ) : filteredRepos.length === 0 ? (
            <div className="py-12 text-center text-gray-500">
              No repositories found
            </div>
          ) : (
            <div className="divide-y divide-gray-100">
              {filteredRepos.map((repo) => (
                <div
                  key={repo.nameWithOwner}
                  className="flex items-center justify-between p-4 hover:bg-gray-50 transition-colors"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-gray-900 truncate">{repo.name}</span>
                      {repo.isPrivate && (
                        <Lock className="h-3 w-3 text-gray-400 flex-shrink-0" />
                      )}
                    </div>
                    <p className="text-xs text-gray-500 truncate">{repo.nameWithOwner}</p>
                    {repo.description && (
                      <p className="text-sm text-gray-500 mt-1 line-clamp-1">{repo.description}</p>
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
        <div className="p-3 border-t border-gray-100 bg-gray-50">
          <p className="text-xs text-gray-500 text-center">
            Press <kbd className="px-1.5 py-0.5 bg-white border border-gray-200 rounded text-gray-600">Esc</kbd> to close
          </p>
        </div>
      </div>
    </div>
  )
}

function CommandPalette({
  onClose,
  repos,
  ports,
  onOpenClone,
  onOpenPort,
  onCopyUrl
}: {
  onClose: () => void
  repos: Repo[]
  ports: Port[]
  onOpenClone: () => void
  onOpenPort: (port: number) => void
  onCopyUrl: (port: number) => void
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
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[15vh] z-50" onClick={onClose}>
      <div
        className="bg-white rounded-xl shadow-2xl w-full max-w-lg overflow-hidden animate-in fade-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="p-3 border-b border-gray-100">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
            <input
              ref={inputRef}
              type="text"
              placeholder="Type a command..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              className="w-full pl-10 pr-4 py-2 text-sm border-0 focus:outline-none focus:ring-0"
            />
          </div>
        </div>
        <div className="max-h-[40vh] overflow-y-auto py-2">
          {filteredCommands.length === 0 ? (
            <div className="py-8 text-center text-gray-500 text-sm">
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
                  ${index === selectedIndex ? 'bg-gray-100' : 'hover:bg-gray-50'}
                `}
              >
                <span className="text-gray-400">{cmd.icon}</span>
                <span className="flex-1">{cmd.label}</span>
                <span className="text-xs text-gray-400">{cmd.category}</span>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diff = date.getTime() - now.getTime()

  if (diff < 0) return 'expired'

  const hours = Math.floor(diff / (1000 * 60 * 60))
  const days = Math.floor(hours / 24)

  if (days > 0) return `${days}d`
  if (hours > 0) return `${hours}h`

  const minutes = Math.floor(diff / (1000 * 60))
  return `${minutes}m`
}

export default App
