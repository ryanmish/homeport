import { useEffect, useState, useCallback, useRef, createContext } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Toaster, toast } from '@/components/ui/sonner'
import { api, type Repo, type Port, type Status, type GitHubRepo, type GitStatus, type RepoInfo, type BranchInfo, type UpdateInfo, type Process, type LogEntry, type ActivityEntry } from '@/lib/api'
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
  Plus,
  Search,
  Check,
  X,
  Terminal,
  GitBranch,
  Share2,
  Timer,
  Trash2,
  Moon,
  Sun,
  Settings,
  AlertCircle,
  MoreHorizontal,
  Play,
  Circle,
  ArrowUp,
  ArrowDown,
  Pencil,
  Filter,
  Download,
  Package,
  ChevronDown,
  RotateCcw,
  Zap,
  Square,
  ScrollText,
  Upload,
  GitCommit,
  History,
  Star,
  Eye,
  EyeOff,
  Loader2,
} from 'lucide-react'

// Convert homeportd repo path to code-server path
const toCodeServerPath = (path: string) => path.replace('/srv/homeport/repos', '/home/coder/repos')

// Theme context
type Theme = 'light' | 'dark'
const ThemeContext = createContext<{ theme: Theme; toggleTheme: () => void }>({
  theme: 'light',
  toggleTheme: () => {},
})

function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [repos, setRepos] = useState<Repo[]>([])
  const [ports, setPorts] = useState<Port[]>([])
  const [showCloneModal, setShowCloneModal] = useState(false)
  const [showNewRepoModal, setShowNewRepoModal] = useState(false)
  const [showCommandPalette, setShowCommandPalette] = useState(false)
  const [showHelpModal, setShowHelpModal] = useState(false)
  const [showSettingsModal, setShowSettingsModal] = useState(false)
  const [loading, setLoading] = useState(true)
  const [theme, setTheme] = useState<Theme>(() => {
    if (typeof window !== 'undefined') {
      return (localStorage.getItem('theme') as Theme) || 'light'
    }
    return 'light'
  })
  const [portHealth, setPortHealth] = useState<Record<number, boolean>>({})
  const [error, setError] = useState<string | null>(null)
  const [repoFilter, setRepoFilter] = useState('')
  const [gitStatuses, setGitStatuses] = useState<Record<string, GitStatus>>({})
  const [showStartCommandModal, setShowStartCommandModal] = useState<Repo | null>(null)
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [repoInfos, setRepoInfos] = useState<Record<string, RepoInfo>>({})
  const [dismissedUpdate, setDismissedUpdate] = useState(false)
  const [processes, setProcesses] = useState<Process[]>([])
  const [processLoading, setProcessLoading] = useState<Record<string, 'starting' | 'stopping'>>({})
  const [showLogsModal, setShowLogsModal] = useState<Repo | null>(null)
  const [showCommitModal, setShowCommitModal] = useState<Repo | null>(null)
  const [activity, setActivity] = useState<ActivityEntry[]>([])
  const [showActivityPanel, setShowActivityPanel] = useState(false)
  const [favorites, setFavorites] = useState<Set<string>>(() => {
    const saved = localStorage.getItem('homeport_favorites')
    return saved ? new Set(JSON.parse(saved)) : new Set()
  })

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

  
  const fetchData = async () => {
    try {
      const [statusData, reposData, portsData, processesData] = await Promise.all([
        api.getStatus(),
        api.getRepos(),
        api.getPorts(),
        api.getProcesses().catch(() => []),
      ])
      setStatus(statusData)
      setRepos(reposData)
      setPorts(portsData)
      setProcesses(processesData)
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

      // Fetch git status and repo info for all repos
      const statuses: Record<string, GitStatus> = {}
      const infos: Record<string, RepoInfo> = {}
      await Promise.all(
        reposData.map(async (repo) => {
          try {
            const [status, info] = await Promise.all([
              api.getRepoStatus(repo.id),
              api.getRepoInfo(repo.id),
            ])
            statuses[repo.id] = status
            infos[repo.id] = info
          } catch {
            // Ignore errors for individual repos
          }
        })
      )
      setGitStatuses(statuses)
      setRepoInfos(infos)

      // Fetch activity log
      try {
        const activityData = await api.getActivity(20)
        setActivity(activityData)
      } catch {
        // Ignore activity fetch errors
      }
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

  // Check for updates on mount (once per session)
  useEffect(() => {
    api.checkForUpdates().then(setUpdateInfo).catch(() => {})
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
        setShowNewRepoModal(false)
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
    toast.success('URL copied to clipboard')
  }

  const copyCurl = (port: number) => {
    const url = `${status?.config.external_url || window.location.origin}/${port}/`
    const curl = `curl -X GET "${url}"`
    navigator.clipboard.writeText(curl)
    toast.success('curl command copied')
  }

  const openPort = (port: number) => {
    window.open(`/${port}/`, '_blank')
  }

  const handleShare = async (port: number, mode: string, password?: string, expiresIn?: string) => {
    try {
      const result = await api.sharePort(port, mode, password, expiresIn)
      toast.success(`Port ${port} shared as ${mode}${result.expires_at ? ' (expires ' + formatRelativeTime(result.expires_at) + ')' : ''}`)
      fetchData()
    } catch (err) {
      toast.error('Failed to share port')
    }
  }

  const handleDeleteRepo = async (repo: Repo) => {
    if (!confirm(`Delete "${repo.name}"? This will remove the repository from disk.`)) {
      return
    }
    try {
      await api.deleteRepo(repo.id)
      toast.success(`Deleted ${repo.name}`)
      fetchData()
    } catch (err) {
      toast.error('Failed to delete repository')
    }
  }

  const handlePullRepo = async (repo: Repo) => {
    try {
      const result = await api.pullRepo(repo.id)
      if (result.success) {
        if (result.message === 'Already up to date') {
          toast.success(`${repo.name}: Already up to date`)
        } else {
          toast.success(`${repo.name}: ${result.files_changed} files changed (+${result.insertions}/-${result.deletions})`)
        }
      } else {
        toast.error(`${repo.name}: ${result.message}`)
      }
      fetchData()
    } catch (err) {
      toast.error(`Failed to pull ${repo.name}`)
    }
  }

  const handlePullAll = async () => {
    toast('Pulling all repositories...')
    let updated = 0
    let upToDate = 0
    let failed = 0
    for (const repo of repos) {
      try {
        const result = await api.pullRepo(repo.id)
        if (result.success) {
          if (result.message === 'Already up to date') {
            upToDate++
          } else {
            updated++
          }
        } else {
          failed++
        }
      } catch (err) {
        failed++
      }
    }
    if (failed > 0) {
      toast.error(`Pull complete: ${updated} updated, ${upToDate} up to date, ${failed} failed`)
    } else {
      toast.success(`Pull complete: ${updated} updated, ${upToDate} already up to date`)
    }
    fetchData()
  }

  const handleUpdateStartCommand = async (repo: Repo, command: string) => {
    try {
      await api.updateRepo(repo.id, { start_command: command })
      toast.success(`Start command updated for ${repo.name}`)
      fetchData()
    } catch (err) {
      toast.error('Failed to update start command')
    }
  }

  const handleExecCommand = async (repo: Repo, command: 'install' | 'fetch' | 'reset') => {
    const commandNames = { install: 'Installing dependencies', fetch: 'Fetching updates', reset: 'Resetting changes' }
    toast(`${commandNames[command]}...`)
    try {
      const result = await api.execCommand(repo.id, command)
      if (result.success) {
        toast.success(`${repo.name}: ${command} completed`)
      } else {
        toast.error(`${repo.name}: ${command} failed`)
      }
      fetchData()
    } catch (err) {
      toast.error(`Failed to run ${command}`)
    }
  }

  const handleCheckoutBranch = async (repo: Repo, branch: string) => {
    try {
      await api.checkoutBranch(repo.id, branch)
      toast.success(`Switched to ${branch}`)
      fetchData()
    } catch (err) {
      toast.error(`Failed to checkout ${branch}`)
    }
  }

  const handleStartProcess = async (repo: Repo) => {
    setProcessLoading(prev => ({ ...prev, [repo.id]: 'starting' }))
    try {
      await api.startProcess(repo.id)
      // Poll until port appears or timeout (15 seconds)
      let attempts = 0
      const pollForPort = async (): Promise<void> => {
        attempts++
        const portsData = await api.getPorts()
        const hasPort = portsData.some(p => p.repo_id === repo.name || p.repo_id === repo.id)
        if (hasPort) {
          await fetchData()
          setProcessLoading(prev => {
            const next = { ...prev }
            delete next[repo.id]
            return next
          })
        } else if (attempts < 15) {
          await new Promise(r => setTimeout(r, 1000))
          return pollForPort()
        } else {
          // Timeout - still refresh and clear loading
          await fetchData()
          setProcessLoading(prev => {
            const next = { ...prev }
            delete next[repo.id]
            return next
          })
        }
      }
      await new Promise(r => setTimeout(r, 500))
      await pollForPort()
    } catch (err) {
      toast.error(`Failed to start: ${err}`)
      setProcessLoading(prev => {
        const next = { ...prev }
        delete next[repo.id]
        return next
      })
    }
  }

  const handleStopProcess = async (repo: Repo) => {
    setProcessLoading(prev => ({ ...prev, [repo.id]: 'stopping' }))
    try {
      await api.stopProcess(repo.id)
      // Poll until port disappears or timeout (15 seconds)
      let attempts = 0
      const pollForStop = async (): Promise<void> => {
        attempts++
        const portsData = await api.getPorts()
        // Check if any port is still associated with this repo
        const hasPort = portsData.some(p => p.repo_id === repo.name || p.repo_id === repo.id)
        if (!hasPort) {
          await fetchData()
          setProcessLoading(prev => {
            const next = { ...prev }
            delete next[repo.id]
            return next
          })
        } else if (attempts < 15) {
          await new Promise(r => setTimeout(r, 1000))
          return pollForStop()
        } else {
          // Timeout - force refresh and clear loading anyway
          await fetchData()
          setProcessLoading(prev => {
            const next = { ...prev }
            delete next[repo.id]
            return next
          })
        }
      }
      await new Promise(r => setTimeout(r, 500))
      await pollForStop()
    } catch (err) {
      toast.error('Failed to stop')
      setProcessLoading(prev => {
        const next = { ...prev }
        delete next[repo.id]
        return next
      })
    }
  }

  const handleGitCommit = async (repo: Repo, message: string) => {
    try {
      const result = await api.gitCommit(repo.id, message)
      if (result.success) {
        toast.success(`Committed: ${result.commit_hash}`)
      } else {
        toast(result.message)
      }
      fetchData()
    } catch (err) {
      toast.error('Commit failed')
    }
  }

  const handleGitPush = async (repo: Repo) => {
    try {
      const result = await api.gitPush(repo.id)
      toast.success(result.message)
      fetchData()
    } catch (err) {
      toast.error('Push failed')
    }
  }

  const toggleFavorite = (repoId: string) => {
    setFavorites(prev => {
      const next = new Set(prev)
      if (next.has(repoId)) {
        next.delete(repoId)
      } else {
        next.add(repoId)
      }
      localStorage.setItem('homeport_favorites', JSON.stringify([...next]))
      return next
    })
  }

  // Get process by repo ID
  const processByRepo = processes.reduce((acc, proc) => {
    acc[proc.repo_id] = proc
    return acc
  }, {} as Record<string, Process>)

  // Sort repos: favorites first, then by name
  const sortedRepos = [...repos].sort((a, b) => {
    const aFav = favorites.has(a.id)
    const bFav = favorites.has(b.id)
    if (aFav && !bFav) return -1
    if (!aFav && bFav) return 1
    return a.name.localeCompare(b.name)
  })

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
        <Toaster position="top-right" />

        {/* Update notification banner */}
        {updateInfo?.update_available && !dismissedUpdate && (
          <div className={`border-b px-4 py-2 flex items-center justify-between ${theme === 'dark' ? 'bg-blue-900/20 border-blue-800/30' : 'bg-blue-50 border-blue-100'}`}>
            <div className="flex items-center gap-2">
              <Download className={`h-4 w-4 ${theme === 'dark' ? 'text-blue-400' : 'text-blue-600'}`} />
              <span className={`text-sm ${theme === 'dark' ? 'text-blue-300' : 'text-blue-700'}`}>
                Homeport {updateInfo.latest_version} is available
              </span>
            </div>
            <div className="flex items-center gap-2">
              {updateInfo.release_url && (
                <a
                  href={updateInfo.release_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`text-sm font-medium ${theme === 'dark' ? 'text-blue-400 hover:text-blue-300' : 'text-blue-600 hover:text-blue-700'}`}
                >
                  View Release
                </a>
              )}
              <button
                onClick={() => setDismissedUpdate(true)}
                className={`p-1 rounded-md ${theme === 'dark' ? 'hover:bg-blue-800/30' : 'hover:bg-blue-100'}`}
              >
                <X className={`h-4 w-4 ${theme === 'dark' ? 'text-blue-400' : 'text-blue-600'}`} />
              </button>
            </div>
          </div>
        )}

        {/* Header */}
        <header className={`sticky top-0 z-40 transition-colors ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}>
          <div className="max-w-6xl mx-auto px-4 sm:px-6 h-14 sm:h-16 flex items-center justify-between">
            {/* Logo and brand */}
            <div className="flex items-center gap-3">
              <div className={`w-9 h-9 rounded-xl flex items-center justify-center ${theme === 'dark' ? 'bg-white text-gray-900' : 'bg-gray-900 text-white'}`}>
                <Logo size={20} />
              </div>
              <div className="flex flex-col">
                <h1 className="text-base font-semibold leading-tight">Homeport</h1>
                <span className={`text-xs font-mono ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                  v{status?.version}
                </span>
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center">
              {/* Search/Command palette trigger */}
              <button
                onClick={() => setShowCommandPalette(true)}
                className={`hidden sm:flex items-center gap-2 h-9 px-3 mr-3 text-sm rounded-lg border transition-colors ${theme === 'dark' ? 'bg-gray-800/50 border-gray-700 text-gray-400 hover:bg-gray-800 hover:border-gray-600' : 'bg-gray-50 border-gray-200 text-gray-500 hover:bg-gray-100 hover:border-gray-300'}`}
              >
                <Search className="h-3.5 w-3.5" />
                <span className="hidden md:inline">Search...</span>
                <kbd className={`hidden md:inline ml-2 px-1.5 py-0.5 text-xs rounded ${theme === 'dark' ? 'bg-gray-700 text-gray-400' : 'bg-gray-200 text-gray-500'}`}>
                  ⌘K
                </kbd>
              </button>

              {/* Icon buttons */}
              <div className={`flex items-center border-r pr-2 mr-2 ${theme === 'dark' ? 'border-gray-800' : 'border-gray-200'}`}>
                <button
                  onClick={() => setShowActivityPanel(!showActivityPanel)}
                  className={`p-2 rounded-lg transition-colors ${showActivityPanel ? theme === 'dark' ? 'bg-gray-800 text-gray-200' : 'bg-gray-100 text-gray-700' : theme === 'dark' ? 'hover:bg-gray-800 text-gray-400 hover:text-gray-200' : 'hover:bg-gray-100 text-gray-500 hover:text-gray-700'}`}
                  title="Activity log"
                >
                  <History className="h-4 w-4" />
                </button>
                <button
                  onClick={toggleTheme}
                  className={`p-2 rounded-lg transition-colors ${theme === 'dark' ? 'hover:bg-gray-800 text-gray-400 hover:text-gray-200' : 'hover:bg-gray-100 text-gray-500 hover:text-gray-700'}`}
                  title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
                >
                  {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
                </button>
                <button
                  onClick={() => setShowSettingsModal(true)}
                  className={`p-2 rounded-lg transition-colors ${theme === 'dark' ? 'hover:bg-gray-800 text-gray-400 hover:text-gray-200' : 'hover:bg-gray-100 text-gray-500 hover:text-gray-700'}`}
                  title="Settings"
                >
                  <Settings className="h-4 w-4" />
                </button>
              </div>

              {/* Primary actions */}
              <div className="flex items-center gap-1.5">
                <Button variant="outline" size="sm" onClick={() => setShowNewRepoModal(true)}>
                  <Plus className="h-4 w-4 sm:mr-1.5" />
                  <span className="hidden sm:inline">New</span>
                </Button>
                <Button size="sm" onClick={() => setShowCloneModal(true)}>
                  <Github className="h-4 w-4 sm:mr-1.5" />
                  <span className="hidden sm:inline">Clone</span>
                </Button>
              </div>
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
            <div className="flex items-center justify-between gap-4">
              <h2 className={`text-sm sm:text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>
                Repositories
              </h2>
              <div className="flex items-center gap-2 flex-1 justify-end">
                {repos.length > 3 && (
                  <div className="relative max-w-xs flex-1">
                    <Filter className={`absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
                    <input
                      type="text"
                      placeholder="Filter..."
                      value={repoFilter}
                      onChange={(e) => setRepoFilter(e.target.value)}
                      className={`w-full pl-8 pr-3 py-1.5 text-sm rounded-lg border focus:outline-none focus:ring-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-100 focus:ring-gray-600 placeholder-gray-500' : 'border-gray-200 focus:ring-gray-400 placeholder-gray-400'}`}
                    />
                  </div>
                )}
                <span className={`text-xs sm:text-sm whitespace-nowrap ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
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
              <Card className={theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'border-gray-200'}>
                <CardContent className="py-10">
                  <div className="text-center space-y-4">
                    <p className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
                      Get started by cloning an existing repository or creating a new one.
                    </p>
                    <div className="flex items-center justify-center gap-3">
                      <Button onClick={() => setShowCloneModal(true)}>
                        <Github className="h-4 w-4 mr-2" />
                        Clone Repository
                      </Button>
                      <Button variant="outline" onClick={() => setShowNewRepoModal(true)}>
                        <Plus className="h-4 w-4 mr-2" />
                        New Repository
                      </Button>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ) : (
              <div className="space-y-3">
                {sortedRepos
                  .filter(repo => !repoFilter || repo.name.toLowerCase().includes(repoFilter.toLowerCase()))
                  .map((repo) => (
                  <RepoCard
                    key={repo.id}
                    repo={repo}
                    ports={portsByRepo[repo.id] || []}
                    portHealth={portHealth}
                    gitStatus={gitStatuses[repo.id]}
                    repoInfo={repoInfos[repo.id]}
                    process={processByRepo[repo.id]}
                    processLoadingState={processLoading[repo.id]}
                    isFavorite={favorites.has(repo.id)}
                    theme={theme}
                    onDelete={() => handleDeleteRepo(repo)}
                    onPull={() => handlePullRepo(repo)}
                    onCopyUrl={copyUrl}
                    onCopyCurl={copyCurl}
                    onOpenPort={openPort}
                    onShare={handleShare}
                    onConfigureStart={() => setShowStartCommandModal(repo)}
                    onExecCommand={(cmd) => handleExecCommand(repo, cmd)}
                    onCheckoutBranch={(branch) => handleCheckoutBranch(repo, branch)}
                    onStartProcess={() => handleStartProcess(repo)}
                    onStopProcess={() => handleStopProcess(repo)}
                    onShowLogs={() => setShowLogsModal(repo)}
                    onCommit={() => setShowCommitModal(repo)}
                    onPush={() => handleGitPush(repo)}
                    onToggleFavorite={() => toggleFavorite(repo.id)}
                  />
                ))}
              </div>
            )}
          </section>

          {/* External Services - filtered to exclude internal services */}
          {(() => {
            const externalPorts = orphanPorts.filter(p =>
              p.process_name !== 'homeportd' && p.port !== 8443 // 8443 is code-server, accessible via /code/
            )
            if (externalPorts.length === 0) return null
            return (
              <section className="space-y-4">
                <div className="flex items-center gap-2">
                  <h2 className={`text-sm sm:text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>
                    External Services
                  </h2>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${theme === 'dark' ? 'bg-gray-800 text-gray-500' : 'bg-gray-100 text-gray-500'}`}>
                    Read-only
                  </span>
                </div>
                <Card className={`${theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'border-gray-200'}`}>
                  <CardContent className="pt-4 space-y-1">
                    {externalPorts.map((port) => (
                      <ExternalPortRow key={port.port} port={port} theme={theme} />
                    ))}
                  </CardContent>
                </Card>
              </section>
            )
          })()}
        </main>

        {/* Modals */}
        {showCloneModal && (
          <CloneModal
            theme={theme}
            onClose={() => setShowCloneModal(false)}
            onClone={async (repo) => {
              await api.cloneRepo(repo)
              setShowCloneModal(false)
              toast.success(`Cloned ${repo}`)
              fetchData()
            }}
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
            onToggleTheme={toggleTheme}
          />
        )}

        {showNewRepoModal && (
          <NewRepoModal
            theme={theme}
            onClose={() => setShowNewRepoModal(false)}
            onCreate={async (name) => {
              await api.initRepo(name)
              toast.success(`Created ${name}`)
              fetchData()
            }}
          />
        )}

        {showStartCommandModal && (
          <StartCommandModal
            theme={theme}
            repo={showStartCommandModal}
            repoInfo={repoInfos[showStartCommandModal.id]}
            onClose={() => setShowStartCommandModal(null)}
            onSave={(command) => handleUpdateStartCommand(showStartCommandModal, command)}
          />
        )}

        {showLogsModal && (
          <LogsModal
            theme={theme}
            repo={showLogsModal}
            onClose={() => setShowLogsModal(null)}
          />
        )}

        {showCommitModal && (
          <CommitModal
            theme={theme}
            repo={showCommitModal}
            onClose={() => setShowCommitModal(null)}
            onCommit={(message) => handleGitCommit(showCommitModal, message)}
          />
        )}

        {/* Activity Panel (slide-out) */}
        {showActivityPanel && (
          <div className="fixed inset-0 z-50" onClick={() => setShowActivityPanel(false)}>
            <div className="absolute inset-0 bg-black/20" />
            <div
              className={`absolute right-0 top-0 h-full w-80 shadow-2xl border-l overflow-hidden animate-in slide-in-from-right duration-200 ${theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'bg-white border-gray-200'}`}
              onClick={(e) => e.stopPropagation()}
            >
              <div className={`p-4 border-b flex items-center justify-between ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
                <h2 className={`font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Activity</h2>
                <button
                  onClick={() => setShowActivityPanel(false)}
                  className={`p-1 rounded-md ${theme === 'dark' ? 'hover:bg-gray-800' : 'hover:bg-gray-100'}`}
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="overflow-y-auto h-full pb-20">
                {activity.length === 0 ? (
                  <div className={`p-4 text-center text-sm ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                    No activity yet
                  </div>
                ) : (
                  <div className="divide-y divide-gray-100 dark:divide-gray-800">
                    {activity.map((entry) => (
                      <div key={entry.id} className={`p-3 ${theme === 'dark' ? 'hover:bg-gray-800/50' : 'hover:bg-gray-50'}`}>
                        <div className="flex items-start gap-3">
                          <div className={`mt-0.5 p-1.5 rounded-lg ${getActivityColor(entry.type, theme)}`}>
                            {getActivityIcon(entry.type)}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className={`text-sm ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                              {entry.message}
                            </p>
                            {entry.repo_name && (
                              <p className={`text-xs truncate ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                                {entry.repo_name}
                              </p>
                            )}
                            {entry.port && entry.port > 0 && (
                              <p className={`text-xs ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                                Port {entry.port}
                              </p>
                            )}
                            <p className={`text-xs mt-1 ${theme === 'dark' ? 'text-gray-600' : 'text-gray-400'}`}>
                              {formatRelativeTime(entry.timestamp)}
                            </p>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>
    </ThemeContext.Provider>
  )
}

function getActivityIcon(type: string) {
  switch (type) {
    case 'clone': return <Download className="h-3 w-3" />
    case 'delete': return <Trash2 className="h-3 w-3" />
    case 'share': return <Share2 className="h-3 w-3" />
    case 'unshare': return <Lock className="h-3 w-3" />
    case 'commit': return <GitCommit className="h-3 w-3" />
    case 'push': return <Upload className="h-3 w-3" />
    case 'pull': return <Download className="h-3 w-3" />
    case 'start': return <Play className="h-3 w-3" />
    case 'stop': return <Square className="h-3 w-3" />
    default: return <History className="h-3 w-3" />
  }
}

function getActivityColor(type: string, theme: Theme) {
  const colors: Record<string, string> = {
    clone: theme === 'dark' ? 'bg-green-900/30 text-green-400' : 'bg-green-50 text-green-600',
    delete: theme === 'dark' ? 'bg-red-900/30 text-red-400' : 'bg-red-50 text-red-600',
    share: theme === 'dark' ? 'bg-blue-900/30 text-blue-400' : 'bg-blue-50 text-blue-600',
    unshare: theme === 'dark' ? 'bg-gray-800 text-gray-400' : 'bg-gray-100 text-gray-600',
    commit: theme === 'dark' ? 'bg-purple-900/30 text-purple-400' : 'bg-purple-50 text-purple-600',
    push: theme === 'dark' ? 'bg-orange-900/30 text-orange-400' : 'bg-orange-50 text-orange-600',
    pull: theme === 'dark' ? 'bg-cyan-900/30 text-cyan-400' : 'bg-cyan-50 text-cyan-600',
    start: theme === 'dark' ? 'bg-emerald-900/30 text-emerald-400' : 'bg-emerald-50 text-emerald-600',
    stop: theme === 'dark' ? 'bg-gray-800 text-gray-400' : 'bg-gray-100 text-gray-600',
  }
  return colors[type] || (theme === 'dark' ? 'bg-gray-800 text-gray-400' : 'bg-gray-100 text-gray-600')
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
    <Card className={`${theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'border-gray-200'}`}>
      <CardContent className="pt-4 sm:pt-5 pb-3 sm:pb-4 px-3 sm:px-6">
        <div className="flex items-start justify-between mb-2 sm:mb-3">
          <div className={`p-1.5 sm:p-2 rounded-lg ${theme === 'dark' ? 'bg-gray-800 text-gray-400' : 'bg-gray-100 text-gray-600'}`}>
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
  gitStatus,
  repoInfo,
  process,
  processLoadingState,
  isFavorite,
  theme,
  onDelete,
  onPull,
  onCopyUrl,
  onCopyCurl,
  onOpenPort,
  onShare,
  onConfigureStart,
  onExecCommand,
  onCheckoutBranch,
  onStartProcess,
  onStopProcess,
  onShowLogs,
  onCommit,
  onPush,
  onToggleFavorite,
}: {
  repo: Repo
  ports: Port[]
  portHealth: Record<number, boolean>
  gitStatus?: GitStatus
  repoInfo?: RepoInfo
  process?: Process
  processLoadingState?: 'starting' | 'stopping'
  isFavorite: boolean
  theme: Theme
  onDelete: () => void
  onPull: () => void
  onCopyUrl: (port: number) => void
  onCopyCurl: (port: number) => void
  onOpenPort: (port: number) => void
  onShare: (port: number, mode: string, password?: string, expiresIn?: string) => void
  onConfigureStart: () => void
  onExecCommand: (cmd: 'install' | 'fetch' | 'reset') => void
  onCheckoutBranch: (branch: string) => void
  onStartProcess: () => void
  onStopProcess: () => void
  onShowLogs: () => void
  onCommit: () => void
  onPush: () => void
  onToggleFavorite: () => void
}) {
  const [showMenu, setShowMenu] = useState(false)
  const [showBranchMenu, setShowBranchMenu] = useState(false)
  const [showQuickActions, setShowQuickActions] = useState(false)
  const [branches, setBranches] = useState<BranchInfo[]>([])
  const menuRef = useRef<HTMLDivElement>(null)
  const branchMenuRef = useRef<HTMLDivElement>(null)
  const quickActionsRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (showBranchMenu) {
      api.getBranches(repo.id, true).then(setBranches).catch(() => {})
    }
  }, [showBranchMenu, repo.id])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false)
      }
      if (branchMenuRef.current && !branchMenuRef.current.contains(e.target as Node)) {
        setShowBranchMenu(false)
      }
      if (quickActionsRef.current && !quickActionsRef.current.contains(e.target as Node)) {
        setShowQuickActions(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <Card className={`${theme === 'dark' ? 'bg-gray-900 border-gray-800' : 'border-gray-200'}`}>
      <CardHeader className={`py-4 border-b rounded-t-xl ${theme === 'dark' ? 'bg-gray-800/30 border-gray-800' : 'bg-gray-50/60 border-gray-100'}`}>
        <div className="flex items-start justify-between gap-4">
          {/* Left side: repo info */}
          <div className="flex items-start gap-3 min-w-0 flex-1">
            <div className={`w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0 ${theme === 'dark' ? 'bg-gray-700' : 'bg-gray-100'}`}>
              <GitBranch className={`h-5 w-5 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-600'}`} />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 mb-0.5">
                <div className="flex items-center gap-2">
                  <button
                    onClick={onToggleFavorite}
                    className={`flex-shrink-0 transition-colors ${isFavorite ? 'text-yellow-500' : theme === 'dark' ? 'text-gray-600 hover:text-gray-400' : 'text-gray-300 hover:text-gray-400'}`}
                    title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
                  >
                    <Star className={`h-4 w-4 ${isFavorite ? 'fill-current' : ''}`} />
                  </button>
                  <CardTitle className="text-base font-semibold truncate">{repo.name}</CardTitle>
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
              <div className="flex items-center gap-2 flex-wrap">
                {gitStatus && (
                  <div className="flex items-center gap-1.5">
                    {/* Branch selector */}
                    <div className="relative" ref={branchMenuRef}>
                      <button
                        onClick={() => setShowBranchMenu(!showBranchMenu)}
                        className={`flex items-center gap-1 text-xs font-mono px-1.5 py-0.5 rounded transition-colors ${theme === 'dark' ? 'bg-gray-700 text-gray-300 hover:bg-gray-600' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}`}
                      >
                        {gitStatus.branch}
                        <ChevronDown className="h-3 w-3" />
                      </button>
                      {showBranchMenu && (
                        <div className={`absolute left-0 top-full mt-1 w-48 max-h-64 overflow-y-auto rounded-lg shadow-lg border z-50 py-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
                          {branches.length === 0 ? (
                            <div className={`px-3 py-2 text-sm ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>Loading...</div>
                          ) : (
                            branches.map((branch) => (
                              <button
                                key={branch.name}
                                onClick={() => { onCheckoutBranch(branch.name); setShowBranchMenu(false); }}
                                className={`flex items-center gap-2 w-full px-3 py-1.5 text-sm text-left ${branch.is_current ? theme === 'dark' ? 'bg-gray-700' : 'bg-gray-100' : ''} ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                              >
                                {branch.is_current && <Check className="h-3 w-3" />}
                                <span className={`flex-1 truncate ${branch.is_current ? '' : 'ml-5'}`}>{branch.name}</span>
                                {branch.is_remote && <span className={`text-xs ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>remote</span>}
                              </button>
                            ))
                          )}
                        </div>
                      )}
                    </div>
                    {gitStatus.is_dirty && (
                      <span className={`flex items-center gap-0.5 text-xs ${theme === 'dark' ? 'text-amber-400' : 'text-amber-600'}`} title="Uncommitted changes">
                        <Circle className="h-2 w-2 fill-current" />
                      </span>
                    )}
                    {gitStatus.ahead > 0 && (
                      <span className={`flex items-center gap-0.5 text-xs ${theme === 'dark' ? 'text-green-400' : 'text-green-600'}`} title={`${gitStatus.ahead} commits ahead`}>
                        <ArrowUp className="h-3 w-3" />
                        {gitStatus.ahead}
                      </span>
                    )}
                    {gitStatus.behind > 0 && (
                      <span className={`flex items-center gap-0.5 text-xs ${theme === 'dark' ? 'text-red-400' : 'text-red-600'}`} title={`${gitStatus.behind} commits behind`}>
                        <ArrowDown className="h-3 w-3" />
                        {gitStatus.behind}
                      </span>
                    )}
                  </div>
                )}
                {/* Needs install badge */}
                {repoInfo?.needs_install && (
                  <button
                    onClick={() => onExecCommand('install')}
                    className={`flex items-center gap-1 text-xs px-1.5 py-0.5 rounded transition-colors ${theme === 'dark' ? 'bg-orange-900/30 text-orange-400 hover:bg-orange-900/50' : 'bg-orange-50 text-orange-600 hover:bg-orange-100'}`}
                    title="Run npm install"
                  >
                    <Package className="h-3 w-3" />
                    needs install
                  </button>
                )}
              </div>
            </div>
          </div>

          {/* Right side: actions */}
          <div className="flex items-center gap-2 flex-shrink-0">
            {/* Quick actions menu */}
            <div className="relative hidden sm:block" ref={quickActionsRef}>
              <Button variant="ghost" size="sm" onClick={() => setShowQuickActions(!showQuickActions)} className="h-8 w-8 p-0" title="Quick actions">
                <Zap className="h-4 w-4" />
              </Button>
              {showQuickActions && (
                <div className={`absolute right-0 top-full mt-1 w-44 rounded-lg shadow-lg border z-50 py-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
                  <button
                    onClick={() => { onExecCommand('install'); setShowQuickActions(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Package className="h-4 w-4" />
                    Install deps
                  </button>
                  <button
                    onClick={() => { onExecCommand('fetch'); setShowQuickActions(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Download className="h-4 w-4" />
                    Git fetch
                  </button>
                  <button
                    onClick={() => { onExecCommand('reset'); setShowQuickActions(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap text-red-600 ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <RotateCcw className="h-4 w-4" />
                    Reset HEAD
                  </button>
                </div>
              )}
            </div>
            <Button variant="ghost" size="sm" onClick={onPull} className="hidden sm:flex h-8 w-8 p-0" title="Pull latest">
              <RefreshCw className="h-4 w-4" />
            </Button>
            <a href={`/code/?folder=${toCodeServerPath(repo.path)}`} target="_blank" rel="noopener noreferrer" className="hidden sm:block">
              <Button variant="outline" size="sm" className="h-8">
                Open in VS Code
              </Button>
            </a>
            <div className="relative" ref={menuRef}>
              <Button variant="ghost" size="sm" onClick={() => setShowMenu(!showMenu)} className="h-8 w-8 p-0">
                <MoreHorizontal className="h-4 w-4" />
              </Button>
              {showMenu && (
                <div className={`absolute right-0 top-full mt-1 w-56 rounded-lg shadow-lg border z-50 py-1 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
                  <a
                    href={`/code/?folder=${toCodeServerPath(repo.path)}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap sm:hidden ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Terminal className="h-4 w-4" />
                    Open in VS Code
                  </a>
                  <button
                    onClick={() => { onPull(); setShowMenu(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap sm:hidden ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <RefreshCw className="h-4 w-4" />
                    Pull Latest
                  </button>
                  {/* Process controls */}
                  {repo.start_command && (
                    process?.status === 'running' ? (
                      <>
                        <button
                          onClick={() => { onStopProcess(); setShowMenu(false); }}
                          className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                        >
                          <Square className="h-4 w-4" />
                          Stop Server
                        </button>
                        <button
                          onClick={() => { onShowLogs(); setShowMenu(false); }}
                          className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                        >
                          <ScrollText className="h-4 w-4" />
                          View Logs
                        </button>
                      </>
                    ) : (
                      <button
                        onClick={() => { onStartProcess(); setShowMenu(false); }}
                        className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                      >
                        <Play className="h-4 w-4" />
                        Start Server
                      </button>
                    )
                  )}
                  {/* Git operations */}
                  {gitStatus?.is_dirty && (
                    <button
                      onClick={() => { onCommit(); setShowMenu(false); }}
                      className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                    >
                      <GitCommit className="h-4 w-4" />
                      Commit Changes
                    </button>
                  )}
                  {gitStatus && gitStatus.ahead > 0 && (
                    <button
                      onClick={() => { onPush(); setShowMenu(false); }}
                      className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                    >
                      <Upload className="h-4 w-4" />
                      Push Changes
                    </button>
                  )}
                  <div className={`my-1 border-t ${theme === 'dark' ? 'border-gray-700' : 'border-gray-200'}`} />
                  <button
                    onClick={() => { onConfigureStart(); setShowMenu(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
                  >
                    <Pencil className="h-4 w-4" />
                    Configure Start Command
                  </button>
                  <button
                    onClick={() => { onDelete(); setShowMenu(false); }}
                    className={`flex items-center gap-2 w-full px-3 py-2 text-sm whitespace-nowrap text-red-600 ${theme === 'dark' ? 'hover:bg-gray-700' : 'hover:bg-gray-100'}`}
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
                isStopping={processLoadingState === 'stopping'}
                theme={theme}
                onCopy={() => onCopyUrl(port.port)}
                onCopyCurl={() => onCopyCurl(port.port)}
                onOpen={() => onOpenPort(port.port)}
                onShare={onShare}
                onStop={onStopProcess}
              />
            ))}
          </div>
        ) : (
          <div className={`flex items-center justify-between py-2 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
            <p className="text-sm">
              {processLoadingState === 'starting' ? 'Starting server...' : repo.start_command ? 'No dev servers running' : 'No start command configured'}
            </p>
            {repo.start_command ? (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={onStartProcess}
                disabled={!!processLoadingState}
              >
                {processLoadingState === 'starting' ? (
                  <Loader2 className="h-3 w-3 mr-1 animate-spin" />
                ) : (
                  <Play className="h-3 w-3 mr-1" />
                )}
                {processLoadingState === 'starting' ? 'Starting...' : 'Start Server'}
              </Button>
            ) : (
              <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onConfigureStart}>
                <Pencil className="h-3 w-3 mr-1" />
                Configure
              </Button>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function PortRow({
  port,
  isHealthy,
  isStopping,
  theme,
  onCopy,
  onCopyCurl,
  onOpen,
  onShare,
  onStop,
}: {
  port: Port
  isHealthy?: boolean
  isStopping?: boolean
  theme: Theme
  onCopy: () => void
  onCopyCurl: () => void
  onOpen: () => void
  onShare: (port: number, mode: string, password?: string, expiresIn?: string) => void
  onStop: () => void
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
        <span
          className={`text-xs sm:text-sm truncate max-w-[120px] sm:max-w-[200px] ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}
          title={port.command || port.process_name || 'Unknown'}
        >
          {(() => {
            const raw = port.command ? port.command.split('/').pop()?.split(' ')[0] : port.process_name
            return raw?.replace(/\s*\(v[\d.]+\)/g, '').replace(/\(v\d+\)$/, '') || 'Unknown'
          })()}
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
      <div className="flex items-center gap-0.5 sm:gap-1">
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
              onCopyUrl={onCopy}
            />
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={onStop}
          disabled={isStopping}
          className={`h-7 w-7 sm:h-8 sm:w-8 p-0 ${theme === 'dark' ? 'text-red-400 hover:text-red-300 hover:bg-red-900/20' : 'text-red-600 hover:text-red-700 hover:bg-red-50'}`}
        >
          {isStopping ? (
            <Loader2 className="h-3.5 w-3.5 sm:h-4 sm:w-4 animate-spin" />
          ) : (
            <Square className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
          )}
        </Button>
      </div>
    </div>
  )
}

function ExternalPortRow({ port, theme }: { port: Port; theme: Theme }) {
  return (
    <div className={`flex items-center justify-between py-2 px-3 rounded-lg ${theme === 'dark' ? 'bg-gray-800/30' : 'bg-gray-100/50'}`}>
      <div className="flex items-center gap-3">
        <div className={`w-2 h-2 rounded-full flex-shrink-0 ${theme === 'dark' ? 'bg-gray-600' : 'bg-gray-400'}`} />
        <code className={`text-xs font-mono px-1.5 py-0.5 rounded ${theme === 'dark' ? 'bg-gray-800 text-gray-500' : 'bg-gray-200 text-gray-500'}`}>
          :{port.port}
        </code>
        <span className={`text-xs ${theme === 'dark' ? 'text-gray-500' : 'text-gray-500'}`}>
          {port.process_name || 'Unknown'}
        </span>
      </div>
      <span className={`text-xs ${theme === 'dark' ? 'text-gray-600' : 'text-gray-400'}`}>
        External
      </span>
    </div>
  )
}

function ShareMenu({
  port,
  theme,
  onShare,
  onClose,
  onCopyUrl
}: {
  port: Port
  theme: Theme
  onShare: (mode: string, password?: string, expiresIn?: string) => void
  onClose: () => void
  onCopyUrl: () => void
}) {
  const [mode, setMode] = useState(port.share_mode)
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)

  const isValid = mode !== 'password' || password.length > 0

  const handleApply = (shouldCopy: boolean) => {
    onShare(mode, mode === 'password' ? password : undefined, undefined)
    if (shouldCopy) {
      onCopyUrl()
    }
  }

  const modeOptions = [
    { value: 'private', title: 'Private', desc: 'Only accessible when logged into Homeport' },
    { value: 'password', title: 'Password', desc: 'Anyone with the password can access' },
    { value: 'public', title: 'Public', desc: 'Anyone with the link can access' },
  ] as const

  return (
    <div className={`absolute right-0 top-full mt-1 w-80 rounded-xl shadow-lg border p-4 z-50 ${theme === 'dark' ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
      <div className="space-y-4">
        <div className={`text-sm font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Share Settings</div>

        {/* Mode options - vertical cards */}
        <div className="space-y-2">
          {modeOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => setMode(opt.value)}
              className={`w-full p-3 rounded-lg border text-left transition-colors ${
                mode === opt.value
                  ? theme === 'dark' ? 'border-blue-500 bg-blue-500/10' : 'border-blue-500 bg-blue-50'
                  : theme === 'dark' ? 'border-gray-700 hover:border-gray-600' : 'border-gray-200 hover:border-gray-300'
              }`}
            >
              <div className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>{opt.title}</div>
              <div className={`text-xs mt-0.5 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{opt.desc}</div>
            </button>
          ))}
        </div>

        {/* Password field */}
        {mode === 'password' && (
          <div>
            <label className={`text-xs font-medium mb-2 block ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Password</label>
            <div className="relative">
              <input
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter password..."
                className={`w-full px-3 py-2 pr-10 text-sm border rounded-lg focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-900 border-gray-600 text-gray-100 focus:ring-gray-500' : 'border-gray-200 focus:ring-gray-400'}`}
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className={`absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded ${theme === 'dark' ? 'text-gray-500 hover:text-gray-300' : 'text-gray-400 hover:text-gray-600'}`}
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>
        )}

        {/* Buttons */}
        <div className={`flex justify-end gap-2 pt-3 border-t ${theme === 'dark' ? 'border-gray-700' : 'border-gray-100'}`}>
          <Button variant="outline" size="sm" onClick={onClose} className="text-xs">
            Cancel
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleApply(true)}
            disabled={!isValid}
            className="text-xs"
          >
            Apply & Copy URL
          </Button>
          <Button
            size="sm"
            onClick={() => handleApply(false)}
            disabled={!isValid}
            className="text-xs"
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
}: {
  theme: Theme
  onClose: () => void
  onClone: (repo: string) => Promise<void>
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
      toast.error('Failed to clone repository')
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
      action: () => window.open(`/code/?folder=${toCodeServerPath(r.path)}`, '_blank'),
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
    { keys: ['⌘', 'K'], description: 'Open command palette' },
    { keys: ['?'], description: 'Show keyboard shortcuts' },
    { keys: ['Esc'], description: 'Close modal / cancel' },
  ]

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Keyboard Shortcuts</h2>
        </div>
        <div className="p-4 space-y-3">
          {shortcuts.map((shortcut, i) => (
            <div key={i} className={`flex items-center justify-between p-3 rounded-lg ${theme === 'dark' ? 'bg-gray-800/50' : 'bg-gray-50'}`}>
              <span className={`text-sm ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>{shortcut.description}</span>
              <div className="flex items-center gap-1">
                {shortcut.keys.map((key, j) => (
                  <kbd
                    key={j}
                    className={`px-2 py-1 text-xs font-mono rounded ${theme === 'dark' ? 'bg-gray-700 text-gray-300' : 'bg-white border border-gray-200 text-gray-600'}`}
                  >
                    {key}
                  </kbd>
                ))}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function SettingsModal({
  theme,
  status,
  onClose,
  onToggleTheme,
}: {
  theme: Theme
  status: Status | null
  onClose: () => void
  onToggleTheme: () => void
}) {
  const [githubStatus, setGithubStatus] = useState<{
    authenticated: boolean
    user?: { login: string; name: string; email: string; avatarUrl: string }
  } | null>(null)
  const [loading, setLoading] = useState(true)
  const [showPasswordChange, setShowPasswordChange] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [changingPassword, setChangingPassword] = useState(false)

  useEffect(() => {
    api.getGitHubStatus()
      .then(setGithubStatus)
      .finally(() => setLoading(false))
  }, [])

  const handlePasswordChange = async () => {
    setPasswordError('')

    if (newPassword.length < 8) {
      setPasswordError('New password must be at least 8 characters')
      return
    }

    if (newPassword !== confirmPassword) {
      setPasswordError('Passwords do not match')
      return
    }

    setChangingPassword(true)
    try {
      const result = await api.changePassword(currentPassword, newPassword)
      toast.success(result.message)
      setShowPasswordChange(false)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      setPasswordError('Current password is incorrect')
    } finally {
      setChangingPassword(false)
    }
  }

  const handleLogout = () => {
    window.location.href = '/logout'
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Settings</h2>
        </div>
        <div className="max-h-[80vh] overflow-y-auto">
          <div className="p-4 space-y-4">
            {/* GitHub Account */}
            <div>
              <label className={`text-xs font-medium uppercase tracking-wide ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                GitHub Account
              </label>
              {loading ? (
                <div className={`mt-2 h-16 rounded-lg animate-pulse ${theme === 'dark' ? 'bg-gray-800' : 'bg-gray-100'}`} />
              ) : githubStatus?.authenticated && githubStatus.user ? (
                <a
                  href={`https://github.com/${githubStatus.user.login}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`mt-2 flex items-center gap-3 p-3 rounded-lg transition-colors ${theme === 'dark' ? 'bg-gray-800/50 hover:bg-gray-800' : 'bg-gray-50 hover:bg-gray-100'}`}
                >
                  <img
                    src={githubStatus.user.avatarUrl}
                    alt={githubStatus.user.login}
                    className="w-10 h-10 rounded-full"
                  />
                  <div className="flex-1 min-w-0">
                    <p className={`font-medium truncate ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>
                      {githubStatus.user.name || githubStatus.user.login}
                    </p>
                    <p className={`text-sm truncate ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
                      @{githubStatus.user.login}
                    </p>
                  </div>
                  <ExternalLink className={`h-4 w-4 ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`} />
                </a>
              ) : (
                <div className={`mt-2 p-3 rounded-lg text-sm ${theme === 'dark' ? 'bg-gray-800/50 text-gray-400' : 'bg-gray-50 text-gray-500'}`}>
                  Not connected. Run <code className={`px-1 py-0.5 rounded ${theme === 'dark' ? 'bg-gray-700' : 'bg-gray-200'}`}>gh auth login</code> to connect.
                </div>
              )}
            </div>

            {/* Appearance */}
            <div>
              <label className={`text-xs font-medium uppercase tracking-wide ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                Appearance
              </label>
              <div className={`mt-2 divide-y rounded-lg overflow-hidden ${theme === 'dark' ? 'bg-gray-800/50 divide-gray-700/50' : 'bg-gray-50 divide-gray-100'}`}>
                <button
                  onClick={onToggleTheme}
                  className={`w-full flex items-center justify-between p-3 transition-colors ${theme === 'dark' ? 'hover:bg-gray-800' : 'hover:bg-gray-100'}`}
                >
                  <div className="flex items-center gap-3">
                    {theme === 'dark' ? <Moon className="h-4 w-4 text-gray-400" /> : <Sun className="h-4 w-4 text-gray-500" />}
                    <span className={`text-sm ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>Theme</span>
                  </div>
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>
                    {theme === 'dark' ? 'Dark' : 'Light'}
                  </span>
                </button>
              </div>
            </div>

            {/* Security */}
            <div>
              <label className={`text-xs font-medium uppercase tracking-wide ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                Security
              </label>
              <div className={`mt-2 divide-y rounded-lg overflow-hidden ${theme === 'dark' ? 'bg-gray-800/50 divide-gray-700/50' : 'bg-gray-50 divide-gray-100'}`}>
                {!showPasswordChange ? (
                  <>
                    <button
                      onClick={() => setShowPasswordChange(true)}
                      className={`w-full flex items-center justify-between p-3 transition-colors ${theme === 'dark' ? 'hover:bg-gray-800' : 'hover:bg-gray-100'}`}
                    >
                      <div className="flex items-center gap-3">
                        <Lock className={`h-4 w-4 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`} />
                        <span className={`text-sm ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>Change Password</span>
                      </div>
                    </button>
                    <button
                      onClick={handleLogout}
                      className={`w-full flex items-center gap-3 p-3 transition-colors ${theme === 'dark' ? 'hover:bg-gray-800 text-red-400' : 'hover:bg-gray-100 text-red-500'}`}
                    >
                      <X className="h-4 w-4" />
                      <span className="text-sm">Sign Out</span>
                    </button>
                  </>
                ) : (
                  <div className="p-3 space-y-3">
                    <input
                      type="password"
                      placeholder="Current password"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      className={`w-full px-3 py-2 text-sm rounded-lg border ${theme === 'dark' ? 'bg-gray-900 border-gray-700 text-gray-100 placeholder-gray-500' : 'bg-white border-gray-200 text-gray-900 placeholder-gray-400'}`}
                    />
                    <input
                      type="password"
                      placeholder="New password (min 8 characters)"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      className={`w-full px-3 py-2 text-sm rounded-lg border ${theme === 'dark' ? 'bg-gray-900 border-gray-700 text-gray-100 placeholder-gray-500' : 'bg-white border-gray-200 text-gray-900 placeholder-gray-400'}`}
                    />
                    <input
                      type="password"
                      placeholder="Confirm new password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      className={`w-full px-3 py-2 text-sm rounded-lg border ${theme === 'dark' ? 'bg-gray-900 border-gray-700 text-gray-100 placeholder-gray-500' : 'bg-white border-gray-200 text-gray-900 placeholder-gray-400'}`}
                    />
                    {passwordError && (
                      <p className="text-sm text-red-500">{passwordError}</p>
                    )}
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          setShowPasswordChange(false)
                          setCurrentPassword('')
                          setNewPassword('')
                          setConfirmPassword('')
                          setPasswordError('')
                        }}
                        className={`flex-1 px-3 py-2 text-sm rounded-lg ${theme === 'dark' ? 'bg-gray-800 text-gray-300 hover:bg-gray-700' : 'bg-gray-100 text-gray-700 hover:bg-gray-200'}`}
                      >
                        Cancel
                      </button>
                      <button
                        onClick={handlePasswordChange}
                        disabled={changingPassword}
                        className="flex-1 px-3 py-2 text-sm rounded-lg bg-blue-600 text-white hover:bg-blue-700 disabled:opacity-50"
                      >
                        {changingPassword ? 'Saving...' : 'Save'}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Server Info */}
            <div>
              <label className={`text-xs font-medium uppercase tracking-wide ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                Server
              </label>
              <div className={`mt-2 divide-y rounded-lg overflow-hidden ${theme === 'dark' ? 'bg-gray-800/50 divide-gray-700/50' : 'bg-gray-50 divide-gray-100'}`}>
                <div className="flex justify-between p-3">
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>External URL</span>
                  <span className={`text-sm font-mono ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                    {status?.config.external_url || 'localhost'}
                  </span>
                </div>
                <div className="flex justify-between p-3">
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Port Range</span>
                  <span className={`text-sm font-mono ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                    {status?.config.port_range || '3000-9999'}
                  </span>
                </div>
                <div className="flex justify-between p-3">
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Mode</span>
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                    {status?.config.dev_mode ? 'Development' : 'Production'}
                  </span>
                </div>
                <div className="flex justify-between p-3">
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Version</span>
                  <span className={`text-sm font-mono ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                    {status?.version}
                  </span>
                </div>
                <div className="flex justify-between p-3">
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>Uptime</span>
                  <span className={`text-sm ${theme === 'dark' ? 'text-gray-200' : 'text-gray-700'}`}>
                    {status?.uptime}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function NewRepoModal({
  theme,
  onClose,
  onCreate,
}: {
  theme: Theme
  onClose: () => void
  onCreate: (name: string) => Promise<void>
}) {
  const [name, setName] = useState('')
  const [creating, setCreating] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) return

    setCreating(true)
    try {
      await onCreate(name.trim())
      onClose()
    } catch (err) {
      toast.error('Failed to create repository')
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <form onSubmit={handleSubmit}>
          <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
            <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>New Repository</h2>
          </div>
          <div className="p-4">
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>
              Repository Name
            </label>
            <input
              ref={inputRef}
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="my-project"
              pattern="[a-zA-Z0-9_-]+"
              className={`mt-2 w-full px-3 py-2.5 text-sm border rounded-lg focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-100 focus:ring-gray-600' : 'border-gray-200 focus:ring-gray-900'}`}
            />
            <p className={`mt-2 text-xs ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
              Only letters, numbers, dashes, and underscores allowed.
            </p>
          </div>
          <div className={`p-4 pt-0 flex gap-2`}>
            <Button type="submit" disabled={!name.trim() || creating} className="flex-1">
              {creating ? <RefreshCw className="h-4 w-4 animate-spin" /> : 'Create Repository'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

function StartCommandModal({
  theme,
  repo,
  repoInfo,
  onClose,
  onSave
}: {
  theme: Theme
  repo: Repo
  repoInfo?: RepoInfo
  onClose: () => void
  onSave: (command: string) => void
}) {
  const [command, setCommand] = useState(repo.start_command || repoInfo?.detected_command || '')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(command)
    onClose()
  }

  // Build quick select commands from available scripts + common ones
  const quickCommands: string[] = []

  // Add detected command first if available
  if (repoInfo?.detected_command) {
    quickCommands.push(repoInfo.detected_command)
  }

  // Add package.json scripts (formatted for package manager)
  if (repoInfo?.available_scripts) {
    const pm = repoInfo.package_manager || 'npm'
    const scripts = Object.keys(repoInfo.available_scripts)
    for (const script of scripts.slice(0, 6)) {
      const cmd = script === 'start'
        ? `${pm} start`
        : `${pm} run ${script}`
      if (!quickCommands.includes(cmd)) {
        quickCommands.push(cmd)
      }
    }
  }

  // Fallback common commands if no scripts detected
  if (quickCommands.length === 0) {
    quickCommands.push('npm run dev', 'npm start', 'yarn dev', 'pnpm dev', 'bun dev')
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <form onSubmit={handleSubmit}>
          <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
            <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Start Command</h2>
            <p className={`text-sm mt-1 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{repo.name}</p>
          </div>
          <div className="p-4 space-y-4">
            {/* Detected command suggestion */}
            {repoInfo?.detected_command && !repo.start_command && (
              <div className={`flex items-center gap-2 p-2 rounded-lg ${theme === 'dark' ? 'bg-green-900/20 border border-green-800/30' : 'bg-green-50 border border-green-100'}`}>
                <Zap className={`h-4 w-4 ${theme === 'dark' ? 'text-green-400' : 'text-green-600'}`} />
                <span className={`text-sm ${theme === 'dark' ? 'text-green-300' : 'text-green-700'}`}>
                  Detected: <code className="font-mono">{repoInfo.detected_command}</code>
                </span>
              </div>
            )}
            <div>
              <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>
                Command
              </label>
              <input
                ref={inputRef}
                type="text"
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                placeholder="npm run dev"
                className={`mt-2 w-full px-3 py-2.5 text-sm font-mono border rounded-lg focus:outline-none focus:ring-2 ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-100 focus:ring-gray-600' : 'border-gray-200 focus:ring-gray-900'}`}
              />
            </div>
            <div>
              <label className={`text-xs ${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>
                {repoInfo?.available_scripts ? 'From package.json' : 'Quick select'}
              </label>
              <div className="flex flex-wrap gap-1.5 mt-1.5">
                {quickCommands.slice(0, 6).map(cmd => (
                  <button
                    key={cmd}
                    type="button"
                    onClick={() => setCommand(cmd)}
                    className={`px-2 py-1 text-xs font-mono rounded transition-colors ${
                      command === cmd
                        ? theme === 'dark' ? 'bg-gray-700 text-gray-100' : 'bg-gray-200 text-gray-900'
                        : theme === 'dark' ? 'bg-gray-800 text-gray-400 hover:bg-gray-700' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                    }`}
                  >
                    {cmd}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <div className={`p-4 pt-0 flex gap-2`}>
            <Button type="button" variant="outline" onClick={onClose} className="flex-1">
              Cancel
            </Button>
            <Button type="submit" className="flex-1">
              Save
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

function LogsModal({
  theme,
  repo,
  onClose
}: {
  theme: Theme
  repo: Repo
  onClose: () => void
}) {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const logsEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        const data = await api.getProcessLogs(repo.id, 200)
        setLogs(data)
      } catch {
        // Process might not be running
      } finally {
        setLoading(false)
      }
    }

    fetchLogs()
    const interval = setInterval(fetchLogs, 1000)
    return () => clearInterval(interval)
  }, [repo.id])

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[5vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-3xl h-[80vh] overflow-hidden animate-in fade-in zoom-in-95 duration-200 flex flex-col ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className={`p-4 border-b flex items-center justify-between ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
          <div>
            <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Process Logs</h2>
            <p className={`text-sm ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{repo.name}</p>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className={`flex-1 overflow-y-auto p-4 font-mono text-xs ${theme === 'dark' ? 'bg-gray-950' : 'bg-gray-50'}`}>
          {loading ? (
            <div className={`${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>Loading...</div>
          ) : logs.length === 0 ? (
            <div className={`${theme === 'dark' ? 'text-gray-500' : 'text-gray-400'}`}>No logs available. Process may not be running.</div>
          ) : (
            logs.map((log, i) => (
              <div key={i} className={`py-0.5 ${log.stream === 'stderr' ? 'text-red-400' : theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>
                <span className={`${theme === 'dark' ? 'text-gray-600' : 'text-gray-400'}`}>
                  {new Date(log.time).toLocaleTimeString()}
                </span>
                {' '}
                {log.message}
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      </div>
    </div>
  )
}

function CommitModal({
  theme,
  repo,
  onClose,
  onCommit
}: {
  theme: Theme
  repo: Repo
  onClose: () => void
  onCommit: (message: string) => void
}) {
  const [message, setMessage] = useState('')
  const inputRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (message.trim()) {
      onCommit(message.trim())
      onClose()
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[10vh] z-50 px-4" onClick={onClose}>
      <div
        className={`rounded-xl shadow-2xl w-full max-w-md overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${theme === 'dark' ? 'bg-gray-900' : 'bg-white'}`}
        onClick={(e) => e.stopPropagation()}
      >
        <form onSubmit={handleSubmit}>
          <div className={`p-4 border-b ${theme === 'dark' ? 'border-gray-800' : 'border-gray-100'}`}>
            <h2 className={`text-base font-semibold ${theme === 'dark' ? 'text-gray-100' : 'text-gray-900'}`}>Commit Changes</h2>
            <p className={`text-sm mt-1 ${theme === 'dark' ? 'text-gray-400' : 'text-gray-500'}`}>{repo.name}</p>
          </div>
          <div className="p-4">
            <label className={`text-sm font-medium ${theme === 'dark' ? 'text-gray-300' : 'text-gray-700'}`}>
              Commit message
            </label>
            <textarea
              ref={inputRef}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="What did you change?"
              rows={3}
              className={`mt-2 w-full px-3 py-2.5 text-sm border rounded-lg focus:outline-none focus:ring-2 resize-none ${theme === 'dark' ? 'bg-gray-800 border-gray-700 text-gray-100 focus:ring-gray-600' : 'border-gray-200 focus:ring-gray-900'}`}
            />
          </div>
          <div className={`p-4 pt-0 flex gap-2`}>
            <Button type="button" variant="outline" onClick={onClose} className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={!message.trim()} className="flex-1">
              Commit
            </Button>
          </div>
        </form>
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
