import { useEffect, useState } from 'react'
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
  Plus
} from 'lucide-react'

function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [repos, setRepos] = useState<Repo[]>([])
  const [ports, setPorts] = useState<Port[]>([])
  const [showCloneModal, setShowCloneModal] = useState(false)
  const [loading, setLoading] = useState(true)

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

  const copyUrl = (port: number) => {
    const url = `${status?.config.external_url || window.location.origin}/${port}/`
    navigator.clipboard.writeText(url)
  }

  const openPort = (port: number) => {
    window.open(`/${port}/`, '_blank')
  }

  const handleShare = async (port: number, mode: string) => {
    try {
      if (mode === 'password') {
        const password = prompt('Enter password:')
        if (!password) return
        await api.sharePort(port, mode, password)
      } else {
        await api.sharePort(port, mode)
      }
      fetchData()
    } catch (err) {
      console.error('Failed to share:', err)
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
      <div className="min-h-screen flex items-center justify-center">
        <RefreshCw className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Server className="h-6 w-6" />
            <h1 className="text-xl font-semibold">Homeport</h1>
            <Badge variant="outline">{status?.version}</Badge>
          </div>
          <div className="flex items-center gap-4 text-sm text-muted-foreground">
            <span>Uptime: {status?.uptime}</span>
            <Button variant="outline" size="sm" onClick={() => setShowCloneModal(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Clone Repo
            </Button>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-6 space-y-6">
        {/* System Stats */}
        <div className="grid grid-cols-3 gap-4">
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Cpu className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="text-sm text-muted-foreground">CPU</p>
                  <p className="text-2xl font-semibold">{status?.stats.cpu_percent.toFixed(1)}%</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <Server className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="text-sm text-muted-foreground">Memory</p>
                  <p className="text-2xl font-semibold">{status?.stats.memory_percent.toFixed(1)}%</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <HardDrive className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="text-sm text-muted-foreground">Disk</p>
                  <p className="text-2xl font-semibold">{status?.stats.disk_percent.toFixed(1)}%</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Repos */}
        <div className="space-y-4">
          <h2 className="text-lg font-semibold">Repositories</h2>

          {repos.length === 0 ? (
            <Card>
              <CardContent className="py-8 text-center text-muted-foreground">
                <FolderGit2 className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p>No repositories yet. Clone one to get started.</p>
              </CardContent>
            </Card>
          ) : (
            repos.map((repo) => (
              <Card key={repo.id}>
                <CardHeader className="pb-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <CardTitle className="text-base">{repo.name}</CardTitle>
                      {repo.github_url && (
                        <a
                          href={repo.github_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <Github className="h-4 w-4" />
                        </a>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <Button variant="outline" size="sm" onClick={() => api.pullRepo(repo.id)}>
                        <RefreshCw className="h-4 w-4 mr-2" />
                        Pull
                      </Button>
                      <a
                        href={`/code/?folder=${repo.path}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center justify-center rounded-md text-sm font-medium border border-input bg-background hover:bg-accent hover:text-accent-foreground h-8 px-3"
                      >
                        Open in code-server
                      </a>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  {portsByRepo[repo.id]?.length > 0 ? (
                    <div className="space-y-2">
                      {portsByRepo[repo.id].map((port) => (
                        <PortRow
                          key={port.port}
                          port={port}
                          onCopy={() => copyUrl(port.port)}
                          onOpen={() => openPort(port.port)}
                          onShare={(mode) => handleShare(port.port, mode)}
                        />
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">No dev servers running</p>
                  )}
                </CardContent>
              </Card>
            ))
          )}
        </div>

        {/* Orphan Ports */}
        {orphanPorts.length > 0 && (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Other Ports</h2>
            <Card>
              <CardContent className="pt-6 space-y-2">
                {orphanPorts.map((port) => (
                  <PortRow
                    key={port.port}
                    port={port}
                    onCopy={() => copyUrl(port.port)}
                    onOpen={() => openPort(port.port)}
                    onShare={(mode) => handleShare(port.port, mode)}
                  />
                ))}
              </CardContent>
            </Card>
          </div>
        )}
      </main>

      {/* Clone Modal */}
      {showCloneModal && (
        <CloneModal
          onClose={() => setShowCloneModal(false)}
          onClone={async (repo) => {
            await api.cloneRepo(repo)
            setShowCloneModal(false)
            fetchData()
          }}
        />
      )}
    </div>
  )
}

function PortRow({
  port,
  onCopy,
  onOpen,
  onShare
}: {
  port: Port
  onCopy: () => void
  onOpen: () => void
  onShare: (mode: string) => void
}) {
  const ShareIcon = port.share_mode === 'public' ? Unlock :
                    port.share_mode === 'password' ? KeyRound : Lock

  return (
    <div className="flex items-center justify-between py-2 px-3 rounded-md bg-muted/50">
      <div className="flex items-center gap-3">
        <code className="text-sm font-mono bg-background px-2 py-1 rounded">
          :{port.port}
        </code>
        <span className="text-sm text-muted-foreground">
          {port.process_name}
        </span>
        <Badge variant={
          port.share_mode === 'public' ? 'success' :
          port.share_mode === 'password' ? 'warning' : 'secondary'
        }>
          <ShareIcon className="h-3 w-3 mr-1" />
          {port.share_mode}
        </Badge>
      </div>
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={onCopy}>
          <Copy className="h-4 w-4" />
        </Button>
        <Button variant="ghost" size="sm" onClick={onOpen}>
          <ExternalLink className="h-4 w-4" />
        </Button>
        <select
          className="text-sm border rounded px-2 py-1"
          value={port.share_mode}
          onChange={(e) => onShare(e.target.value)}
        >
          <option value="private">Private</option>
          <option value="password">Password</option>
          <option value="public">Public</option>
        </select>
      </div>
    </div>
  )
}

function CloneModal({
  onClose,
  onClone
}: {
  onClose: () => void
  onClone: (repo: string) => Promise<void>
}) {
  const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([])
  const [loading, setLoading] = useState(true)
  const [cloning, setCloning] = useState<string | null>(null)

  useEffect(() => {
    api.getGitHubRepos(50).then(setGithubRepos).finally(() => setLoading(false))
  }, [])

  const handleClone = async (repo: string) => {
    setCloning(repo)
    try {
      await onClone(repo)
    } catch (err) {
      console.error('Clone failed:', err)
    } finally {
      setCloning(null)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-background rounded-lg shadow-lg w-full max-w-2xl max-h-[80vh] overflow-hidden">
        <div className="p-4 border-b flex items-center justify-between">
          <h2 className="text-lg font-semibold">Clone Repository</h2>
          <Button variant="ghost" size="sm" onClick={onClose}>Close</Button>
        </div>
        <div className="p-4 overflow-y-auto max-h-[60vh]">
          {loading ? (
            <div className="flex justify-center py-8">
              <RefreshCw className="h-6 w-6 animate-spin" />
            </div>
          ) : (
            <div className="space-y-2">
              {githubRepos.map((repo) => (
                <div
                  key={repo.nameWithOwner}
                  className="flex items-center justify-between p-3 rounded-md hover:bg-muted"
                >
                  <div>
                    <p className="font-medium">{repo.name}</p>
                    <p className="text-sm text-muted-foreground">{repo.nameWithOwner}</p>
                    {repo.description && (
                      <p className="text-sm text-muted-foreground mt-1 line-clamp-1">{repo.description}</p>
                    )}
                  </div>
                  <Button
                    size="sm"
                    onClick={() => handleClone(repo.nameWithOwner)}
                    disabled={cloning !== null}
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

export default App
