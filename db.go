package main

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Worktree struct {
	ID        int
	RepoName  string
	Branch    string
	Path      string
	CreatedAt time.Time
}

type DB struct{ *sql.DB }

func OpenDB() (*DB, error) {
	conn, err := sql.Open("sqlite", dbPath())
	if err != nil {
		return nil, err
	}
	_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS worktrees (
		id INTEGER PRIMARY KEY,
		repo_name TEXT NOT NULL,
		branch TEXT NOT NULL,
		path TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}
	return &DB{conn}, nil
}

func (d *DB) Insert(w Worktree) error {
	_, err := d.Exec(`INSERT INTO worktrees (repo_name, branch, path) VALUES (?,?,?)`,
		w.RepoName, w.Branch, w.Path)
	return err
}

func (d *DB) Delete(repoName, branch string) error {
	_, err := d.Exec(`DELETE FROM worktrees WHERE repo_name=? AND branch=?`, repoName, branch)
	return err
}

func (d *DB) List() ([]Worktree, error) {
	return d.query(`SELECT id, repo_name, branch, path, created_at FROM worktrees ORDER BY created_at DESC`)
}

func (d *DB) ListByRepo(repoName string) ([]Worktree, error) {
	return d.query(`SELECT id, repo_name, branch, path, created_at FROM worktrees WHERE repo_name=? ORDER BY created_at DESC`, repoName)
}

func (d *DB) query(q string, args ...any) ([]Worktree, error) {
	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Worktree
	for rows.Next() {
		var w Worktree
		if err := rows.Scan(&w.ID, &w.RepoName, &w.Branch, &w.Path, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
