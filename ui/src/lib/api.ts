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
  share_mode: 'private' | 'password' | 'public'
  first_seen: string
  last_seen: string
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
    fetchJSON<{ status: string }>(`/repos/${id}/pull`, { method: 'POST' }),

  getGitHubRepos: (limit = 100) =>
    fetchJSON<GitHubRepo[]>(`/github/repos?limit=${limit}`),

  getGitHubStatus: () =>
    fetchJSON<{ authenticated: boolean }>('/github/status'),

  sharePort: (port: number, mode: string, password?: string) =>
    fetchJSON<{ status: string; mode: string; url: string }>(`/share/${port}`, {
      method: 'POST',
      body: JSON.stringify({ mode, password }),
    }),

  unsharePort: (port: number) =>
    fetchJSON<{ status: string }>(`/share/${port}`, { method: 'DELETE' }),
}
