package store

import (
	"database/sql"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS repos (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL,
			github_url TEXT,
			start_command TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS ports (
			port INTEGER PRIMARY KEY,
			repo_id TEXT REFERENCES repos(id),
			pid INTEGER,
			process_name TEXT,
			share_mode TEXT DEFAULT 'private',
			password_hash TEXT,
			expires_at TIMESTAMP,
			first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// Migration: add expires_at column if it doesn't exist
		`ALTER TABLE ports ADD COLUMN expires_at TIMESTAMP`,
		`CREATE TABLE IF NOT EXISTS access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			port INTEGER,
			ip TEXT,
			user_agent TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			authenticated BOOLEAN
		)`,
		`CREATE TABLE IF NOT EXISTS terminal_sessions (
			id TEXT PRIMARY KEY,
			repo_id TEXT NOT NULL,
			repo_path TEXT NOT NULL,
			pid INTEGER,
			title TEXT,
			status TEXT DEFAULT 'running',
			created_at TIMESTAMP NOT NULL,
			last_used TIMESTAMP NOT NULL
		)`,
	}

	for _, m := range migrations {
		_, err := s.db.Exec(m)
		// Ignore "duplicate column" errors from ALTER TABLE migrations
		if err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}

	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}

// Repo operations

func (s *Store) ListRepos() ([]Repo, error) {
	rows, err := s.db.Query(`SELECT id, name, path, github_url, start_command, created_at, updated_at FROM repos ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []Repo
	for rows.Next() {
		var r Repo
		var githubURL, startCmd sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &githubURL, &startCmd, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.GitHubURL = githubURL.String
		r.StartCommand = startCmd.String
		repos = append(repos, r)
	}
	return repos, nil
}

func (s *Store) GetRepo(id string) (*Repo, error) {
	var r Repo
	var githubURL, startCmd sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, path, github_url, start_command, created_at, updated_at FROM repos WHERE id = ?`,
		id,
	).Scan(&r.ID, &r.Name, &r.Path, &githubURL, &startCmd, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.GitHubURL = githubURL.String
	r.StartCommand = startCmd.String
	return &r, nil
}

func (s *Store) CreateRepo(r *Repo) error {
	_, err := s.db.Exec(
		`INSERT INTO repos (id, name, path, github_url, start_command, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Path, r.GitHubURL, r.StartCommand, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

func (s *Store) DeleteRepo(id string) error {
	_, err := s.db.Exec(`DELETE FROM repos WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateRepo(r *Repo) error {
	_, err := s.db.Exec(
		`UPDATE repos SET name = ?, path = ?, github_url = ?, start_command = ?, updated_at = ? WHERE id = ?`,
		r.Name, r.Path, r.GitHubURL, r.StartCommand, r.UpdatedAt, r.ID,
	)
	return err
}

func (s *Store) GetRepoByPath(path string) (*Repo, error) {
	var r Repo
	var githubURL, startCmd sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, path, github_url, start_command, created_at, updated_at FROM repos WHERE path = ?`,
		path,
	).Scan(&r.ID, &r.Name, &r.Path, &githubURL, &startCmd, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.GitHubURL = githubURL.String
	r.StartCommand = startCmd.String
	return &r, nil
}

// Port operations

func (s *Store) UpsertPort(p *Port) error {
	_, err := s.db.Exec(`
		INSERT INTO ports (port, repo_id, pid, process_name, share_mode, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(port) DO UPDATE SET
			repo_id = excluded.repo_id,
			pid = excluded.pid,
			process_name = excluded.process_name,
			last_seen = excluded.last_seen
	`, p.Port, p.RepoID, p.PID, p.ProcessName, p.ShareMode, p.FirstSeen, p.LastSeen)
	return err
}

func (s *Store) GetPort(port int) (*Port, error) {
	var p Port
	var repoID sql.NullString
	var pid sql.NullInt64
	var processName sql.NullString
	var passwordHash sql.NullString
	var expiresAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT port, repo_id, pid, process_name, share_mode, password_hash, expires_at, first_seen, last_seen
		FROM ports WHERE port = ?
	`, port).Scan(&p.Port, &repoID, &pid, &processName, &p.ShareMode, &passwordHash, &expiresAt, &p.FirstSeen, &p.LastSeen)
	if err != nil {
		return nil, err
	}
	p.RepoID = repoID.String
	p.PID = int(pid.Int64)
	p.ProcessName = processName.String
	p.PasswordHash = passwordHash.String
	if expiresAt.Valid {
		p.ExpiresAt = &expiresAt.Time
	}
	return &p, nil
}

func (s *Store) ListPorts() ([]Port, error) {
	rows, err := s.db.Query(`
		SELECT p.port, p.repo_id, r.name, p.pid, p.process_name, p.share_mode, p.expires_at, p.first_seen, p.last_seen
		FROM ports p
		LEFT JOIN repos r ON p.repo_id = r.id
		ORDER BY p.port
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ports []Port
	for rows.Next() {
		var p Port
		var repoID, repoName, processName sql.NullString
		var pid sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(&p.Port, &repoID, &repoName, &pid, &processName, &p.ShareMode, &expiresAt, &p.FirstSeen, &p.LastSeen); err != nil {
			return nil, err
		}
		p.RepoID = repoID.String
		p.RepoName = repoName.String
		p.PID = int(pid.Int64)
		p.ProcessName = processName.String
		if expiresAt.Valid {
			p.ExpiresAt = &expiresAt.Time
		}
		ports = append(ports, p)
	}
	return ports, nil
}

func (s *Store) UpdatePortShare(port int, mode string, passwordHash string, expiresAt *time.Time) error {
	_, err := s.db.Exec(`UPDATE ports SET share_mode = ?, password_hash = ?, expires_at = ? WHERE port = ?`, mode, passwordHash, expiresAt, port)
	return err
}

func (s *Store) DeletePort(port int) error {
	_, err := s.db.Exec(`DELETE FROM ports WHERE port = ?`, port)
	return err
}

func (s *Store) CleanupStalePorts(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM ports WHERE last_seen < ?`, before)
	return err
}

// Access log operations

func (s *Store) LogAccess(port int, ip string, userAgent string, authenticated bool) error {
	_, err := s.db.Exec(`INSERT INTO access_logs (port, ip, user_agent, authenticated) VALUES (?, ?, ?, ?)`,
		port, ip, userAgent, authenticated)
	return err
}

func (s *Store) GetAccessLogs(port int, limit int) ([]AccessLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, port, ip, user_agent, timestamp, authenticated FROM access_logs WHERE port = ? ORDER BY timestamp DESC LIMIT ?`, port, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AccessLog
	for rows.Next() {
		var log AccessLog
		if err := rows.Scan(&log.ID, &log.Port, &log.IP, &log.UserAgent, &log.Timestamp, &log.Authenticated); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (s *Store) GetAllAccessLogs(limit int) ([]AccessLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`SELECT id, port, ip, user_agent, timestamp, authenticated FROM access_logs ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []AccessLog
	for rows.Next() {
		var log AccessLog
		if err := rows.Scan(&log.ID, &log.Port, &log.IP, &log.UserAgent, &log.Timestamp, &log.Authenticated); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// Terminal session operations

func (s *Store) SaveTerminalSession(sess *TerminalSession) error {
	_, err := s.db.Exec(`
		INSERT INTO terminal_sessions (id, repo_id, repo_path, pid, title, status, created_at, last_used)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			pid = excluded.pid,
			title = excluded.title,
			status = excluded.status,
			last_used = excluded.last_used
	`, sess.ID, sess.RepoID, sess.RepoPath, sess.PID, sess.Title, sess.Status, sess.CreatedAt, sess.LastUsed)
	return err
}

func (s *Store) GetTerminalSession(id string) (*TerminalSession, error) {
	var sess TerminalSession
	var pid sql.NullInt64
	var title sql.NullString
	err := s.db.QueryRow(`
		SELECT id, repo_id, repo_path, pid, title, status, created_at, last_used
		FROM terminal_sessions WHERE id = ?
	`, id).Scan(&sess.ID, &sess.RepoID, &sess.RepoPath, &pid, &title, &sess.Status, &sess.CreatedAt, &sess.LastUsed)
	if err != nil {
		return nil, err
	}
	sess.PID = int(pid.Int64)
	sess.Title = title.String
	return &sess, nil
}

func (s *Store) ListTerminalSessions() ([]TerminalSession, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_id, repo_path, pid, title, status, created_at, last_used
		FROM terminal_sessions ORDER BY last_used DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []TerminalSession
	for rows.Next() {
		var sess TerminalSession
		var pid sql.NullInt64
		var title sql.NullString
		if err := rows.Scan(&sess.ID, &sess.RepoID, &sess.RepoPath, &pid, &title, &sess.Status, &sess.CreatedAt, &sess.LastUsed); err != nil {
			return nil, err
		}
		sess.PID = int(pid.Int64)
		sess.Title = title.String
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (s *Store) ListTerminalSessionsByRepo(repoID string) ([]TerminalSession, error) {
	rows, err := s.db.Query(`
		SELECT id, repo_id, repo_path, pid, title, status, created_at, last_used
		FROM terminal_sessions WHERE repo_id = ? ORDER BY last_used DESC
	`, repoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []TerminalSession
	for rows.Next() {
		var sess TerminalSession
		var pid sql.NullInt64
		var title sql.NullString
		if err := rows.Scan(&sess.ID, &sess.RepoID, &sess.RepoPath, &pid, &title, &sess.Status, &sess.CreatedAt, &sess.LastUsed); err != nil {
			return nil, err
		}
		sess.PID = int(pid.Int64)
		sess.Title = title.String
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (s *Store) UpdateTerminalSessionStatus(id string, status string) error {
	_, err := s.db.Exec(`UPDATE terminal_sessions SET status = ?, last_used = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

func (s *Store) UpdateTerminalSessionTitle(id string, title string) error {
	_, err := s.db.Exec(`UPDATE terminal_sessions SET title = ?, last_used = ? WHERE id = ?`, title, time.Now(), id)
	return err
}

func (s *Store) UpdateTerminalSessionLastUsed(id string) error {
	_, err := s.db.Exec(`UPDATE terminal_sessions SET last_used = ? WHERE id = ?`, time.Now(), id)
	return err
}

func (s *Store) DeleteTerminalSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM terminal_sessions WHERE id = ?`, id)
	return err
}

func (s *Store) MarkAllTerminalSessionsExited() error {
	_, err := s.db.Exec(`UPDATE terminal_sessions SET status = 'exited' WHERE status = 'running'`)
	return err
}
