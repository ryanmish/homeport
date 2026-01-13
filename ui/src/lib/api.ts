// API types matching Go structs

export interface Repo {
  id: string
  name: string
  path: string
  github_url?: string
  start_command?: string
  created_at: string
  updated_at: string
  ports?: Port[]
}

export interface Port {
  port: number
  repo_id?: string
  repo_name?: string
  pid?: number
  process_name?: string
  command?: string
  share_mode: 'private' | 'password' | 'public'
  expires_at?: string
  first_seen: string
  last_seen: string
}

export interface GitStatus {
  branch: string
  is_dirty: boolean
  ahead: number
  behind: number
  last_commit?: string
  last_commit_msg?: string
}

export interface PullResult {
  success: boolean
  message: string
  files_changed: number
  insertions: number
  deletions: number
}

export interface SystemStats {
  cpu_percent: number
  memory_percent: number
  memory_used_gb: number
  memory_total_gb: number
  disk_percent: number
  disk_used_gb: number
  disk_total_gb: number
}

export interface Status {
  status: string
  version: string
  uptime: string
  stats: SystemStats
  config: {
    port_range: string
    external_url: string
    dev_mode: boolean
  }
}

export interface GitHubRepo {
  name: string
  nameWithOwner: string
  description: string
  url: string
  isPrivate: boolean
  isFork: boolean
}

export interface RepoInfo {
  has_package_json: boolean
  has_node_modules: boolean
  needs_install: boolean
  detected_command?: string
  available_scripts?: Record<string, string>
  package_manager?: string
}

export interface BranchInfo {
  name: string
  is_current: boolean
  is_remote: boolean
}

export interface UpdateInfo {
  update_available: boolean
  current_version: string
  latest_version?: string
  release_url?: string
  release_notes?: string
  error?: string
}

export interface ExecResult {
  success: boolean
  command: string
  output: string
}

export interface Process {
  repo_id: string
  repo_name: string
  command: string
  pid: number
  started_at: string
  status: 'running' | 'stopped' | 'failed'
}

export interface LogEntry {
  time: string
  stream: 'stdout' | 'stderr'
  message: string
}

export interface GitCommitResult {
  success: boolean
  message: string
  commit_hash?: string
}

export interface ActivityEntry {
  id: number
  timestamp: string
  type: string
  repo_id?: string
  repo_name?: string
  port?: number
  message: string
  details?: string
}

// API functions

const API_BASE = '/api'

async function fetchJSON<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })
  if (!res.ok) {
    throw new Error(`API error: ${res.status}`)
  }
  return res.json()
}

export const api = {
  getStatus: () => fetchJSON<Status>('/status'),

  getPorts: () => fetchJSON<Port[]>('/ports'),

  getRepos: () => fetchJSON<Repo[]>('/repos'),

  cloneRepo: (repo: string) =>
    fetchJSON<Repo>('/repos', {
      method: 'POST',
      body: JSON.stringify({ repo }),
    }),

  deleteRepo: (id: string) =>
    fetch(API_BASE + `/repos/${id}`, { method: 'DELETE' }),

  pullRepo: (id: string) =>
    fetchJSON<PullResult>(`/repos/${id}/pull`, { method: 'POST' }),

  getRepoStatus: (id: string) =>
    fetchJSON<GitStatus>(`/repos/${id}/status`),

  updateRepo: (id: string, data: { start_command?: string }) =>
    fetchJSON<Repo>(`/repos/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  getGitHubRepos: (limit = 100) =>
    fetchJSON<GitHubRepo[]>(`/github/repos?limit=${limit}`),

  getGitHubStatus: () =>
    fetchJSON<{
      authenticated: boolean
      user?: {
        login: string
        name: string
        email: string
        avatarUrl: string
      }
    }>('/github/status'),

  initRepo: (name: string) =>
    fetchJSON<Repo>('/repos/init', {
      method: 'POST',
      body: JSON.stringify({ name }),
    }),

  sharePort: (port: number, mode: string, password?: string, expiresIn?: string) =>
    fetchJSON<{ status: string; mode: string; url: string; expires_at?: string }>(`/share/${port}`, {
      method: 'POST',
      body: JSON.stringify({ mode, password, expires_in: expiresIn }),
    }),

  unsharePort: (port: number) =>
    fetchJSON<{ status: string }>(`/share/${port}`, { method: 'DELETE' }),

  searchGitHubRepos: (query: string, limit = 20) =>
    fetchJSON<GitHubRepo[]>(`/github/search?q=${encodeURIComponent(query)}&limit=${limit}`),

  // New endpoints for repo info, branches, and actions
  getRepoInfo: (id: string) =>
    fetchJSON<RepoInfo>(`/repos/${id}/info`),

  getBranches: (id: string, includeRemote = false) =>
    fetchJSON<BranchInfo[]>(`/repos/${id}/branches${includeRemote ? '?include_remote=true' : ''}`),

  checkoutBranch: (id: string, branch: string) =>
    fetchJSON<{ status: string; branch: string; message: string }>(`/repos/${id}/checkout`, {
      method: 'POST',
      body: JSON.stringify({ branch }),
    }),

  execCommand: (id: string, command: 'install' | 'fetch' | 'reset') =>
    fetchJSON<ExecResult>(`/repos/${id}/exec`, {
      method: 'POST',
      body: JSON.stringify({ command }),
    }),

  checkForUpdates: () =>
    fetchJSON<UpdateInfo>('/updates'),

  // Process management
  getProcesses: () =>
    fetchJSON<Process[]>('/processes'),

  startProcess: (repoId: string) =>
    fetchJSON<Process>(`/processes/${repoId}/start`, { method: 'POST' }),

  stopProcess: (repoId: string) =>
    fetchJSON<{ status: string }>(`/processes/${repoId}/stop`, { method: 'POST' }),

  getProcessLogs: (repoId: string, limit = 100) =>
    fetchJSON<LogEntry[]>(`/processes/${repoId}/logs?limit=${limit}`),

  // Git operations
  gitCommit: (repoId: string, message: string) =>
    fetchJSON<GitCommitResult>(`/repos/${repoId}/commit`, {
      method: 'POST',
      body: JSON.stringify({ message }),
    }),

  gitPush: (repoId: string) =>
    fetchJSON<{ success: boolean; message: string }>(`/repos/${repoId}/push`, {
      method: 'POST',
    }),

  // Activity log
  getActivity: (limit = 50) =>
    fetchJSON<ActivityEntry[]>(`/activity?limit=${limit}`),
}
