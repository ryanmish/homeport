package store

import (
	"database/sql"
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
			first_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			port INTEGER,
			ip TEXT,
			user_agent TEXT,
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			authenticated BOOLEAN
		)`,
	}

	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}

	return nil
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
	err := s.db.QueryRow(`
		SELECT port, repo_id, pid, process_name, share_mode, password_hash, first_seen, last_seen
		FROM ports WHERE port = ?
	`, port).Scan(&p.Port, &repoID, &pid, &processName, &p.ShareMode, &passwordHash, &p.FirstSeen, &p.LastSeen)
	if err != nil {
		return nil, err
	}
	p.RepoID = repoID.String
	p.PID = int(pid.Int64)
	p.ProcessName = processName.String
	p.PasswordHash = passwordHash.String
	return &p, nil
}

func (s *Store) ListPorts() ([]Port, error) {
	rows, err := s.db.Query(`
		SELECT p.port, p.repo_id, r.name, p.pid, p.process_name, p.share_mode, p.first_seen, p.last_seen
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
		if err := rows.Scan(&p.Port, &repoID, &repoName, &pid, &processName, &p.ShareMode, &p.FirstSeen, &p.LastSeen); err != nil {
			return nil, err
		}
		p.RepoID = repoID.String
		p.RepoName = repoName.String
		p.PID = int(pid.Int64)
		p.ProcessName = processName.String
		ports = append(ports, p)
	}
	return ports, nil
}

func (s *Store) UpdatePortShare(port int, mode string, passwordHash string) error {
	_, err := s.db.Exec(`UPDATE ports SET share_mode = ?, password_hash = ? WHERE port = ?`, mode, passwordHash, port)
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
