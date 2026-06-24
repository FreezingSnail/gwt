package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Repo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	SymlinkTarget string `json:"symlink_target,omitempty"` // e.g. ~/Library/Application Support/factorio/mods/chiikat
}

type Config struct {
	WorktreeRoot string `json:"worktree_root"`
	Repos        []Repo `json:"repos"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gwt")
}

func configPath() string { return filepath.Join(configDir(), "config.json") }
func dbPath() string     { return filepath.Join(configDir(), "gwt.db") }

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func LoadConfig() (*Config, error) {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return nil, err
	}
	path := configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := &Config{WorktreeRoot: "~/code/.worktrees"}
		return cfg, cfg.Save()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	return &cfg, json.NewDecoder(f).Decode(&cfg)
}

func (c *Config) Save() error {
	f, err := os.Create(configPath())
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func (c *Config) WorktreeRootAbs() string {
	return expandHome(c.WorktreeRoot)
}

func (c *Config) RepoByName(name string) *Repo {
	for i := range c.Repos {
		if c.Repos[i].Name == name {
			return &c.Repos[i]
		}
	}
	return nil
}

func (c *Config) SetSymlinkTarget(name, target string) error {
	for i := range c.Repos {
		if c.Repos[i].Name == name {
			c.Repos[i].SymlinkTarget = target
			return c.Save()
		}
	}
	return fmt.Errorf("repo %q not found", name)
}

func (c *Config) AddRepo(path, symlinkTarget string) error {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return err
	}
	name := filepath.Base(abs)
	for _, r := range c.Repos {
		if r.Name == name {
			return nil // already exists
		}
	}
	c.Repos = append(c.Repos, Repo{Name: name, Path: abs, SymlinkTarget: symlinkTarget})
	return c.Save()
}
