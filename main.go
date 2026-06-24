package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gwt/tui"
)

func main() {
	args := os.Args[1:]
	cfg, err := LoadConfig()
	if err != nil {
		fatal("config: %v", err)
	}
	db, err := OpenDB()
	if err != nil {
		fatal("db: %v", err)
	}
	defer db.Close()

	if len(args) == 0 {
		launchTUI(cfg, db)
		return
	}

	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printHelp()
		return
	}

	switch args[0] {
	case "repo":
		if len(args) < 2 {
			fatal("usage: gwt repo <add|list>")
		}
		switch args[1] {
		case "add":
			if len(args) < 3 {
				fatal("usage: gwt repo add <path> [symlink_target]")
			}
			symTarget := ""
			if len(args) >= 4 {
				symTarget = args[3]
			}
			if err := cfg.AddRepo(args[2], symTarget); err != nil {
				fatal("%v", err)
			}
			fmt.Println("added", filepath.Base(expandHome(args[2])))
		case "list":
			for _, r := range cfg.Repos {
				sym := r.SymlinkTarget
				if sym == "" {
					sym = "-"
				}
				fmt.Printf("%-20s %-40s %s\n", r.Name, r.Path, sym)
			}
		case "set-symlink":
			if len(args) < 4 {
				fatal("usage: gwt repo set-symlink <repo> <symlink_target>")
			}
			if err := cfg.SetSymlinkTarget(args[2], args[3]); err != nil {
				fatal("%v", err)
			}
			fmt.Printf("symlink_target for %s set to %s\n", args[2], args[3])
		default:
			fatal("unknown: gwt repo %s", args[1])
		}

	case "create":
		if len(args) < 3 {
			fatal("usage: gwt create <repo> <branch>")
		}
		if err := cmdCreate(cfg, db, args[1], args[2]); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("created %s/%s\n", args[1], args[2])

	case "delete":
		if len(args) < 3 {
			fatal("usage: gwt delete <repo> <branch>")
		}
		if err := cmdDelete(cfg, db, args[1], args[2]); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("deleted %s/%s\n", args[1], args[2])

	case "push":
		if len(args) < 3 {
			fatal("usage: gwt push <repo> <branch>")
		}
		if err := cmdPush(cfg, args[1], args[2]); err != nil {
			fatal("%v", err)
		}
		fmt.Printf("pushed %s/%s\n", args[1], args[2])

	case "symlink":
		if len(args) < 2 {
			fatal("usage: gwt symlink <set|status>")
		}
		switch args[1] {
		case "set":
			if len(args) < 4 {
				fatal("usage: gwt symlink set <repo> <branch|main>")
			}
			if err := cmdSymlink(cfg, args[2], args[3]); err != nil {
				fatal("%v", err)
			}
			fmt.Printf("symlink %s → %s\n", args[2], args[3])
		case "status":
			for _, r := range cfg.Repos {
				if r.SymlinkTarget == "" {
					continue
				}
				cur := SymlinkStatus(&r)
				if cur == "" {
					cur = "(unset)"
				}
				fmt.Printf("%-20s %s → %s\n", r.Name, r.SymlinkTarget, cur)
			}
		default:
			fatal("unknown: gwt symlink %s", args[1])
		}

	case "list":
		wts, err := db.List()
		if err != nil {
			fatal("%v", err)
		}
		for _, w := range wts {
			fmt.Printf("%-20s %-30s %s\n", w.RepoName, w.Branch, w.Path)
		}

	default:
		fatal("unknown command: %s\nrun 'gwt -h' for usage", args[0])
	}
}

func printHelp() {
	fmt.Print(`gwt — git worktree manager

USAGE
  gwt                                   launch TUI
  gwt -h                                show this help

CLI COMMANDS
  gwt list                              list all worktrees
  gwt create <repo> <branch>            create worktree + push -u
  gwt delete <repo> <branch>            remove worktree + cleanup
  gwt push   <repo> <branch>            push -u origin <branch>

  gwt repo add <path> [symlink_target]  register repo (symlink_target = mods folder path)
  gwt repo set-symlink <repo> <path>    set/update symlink_target for registered repo
  gwt repo list                         list registered repos

  gwt symlink set    <repo> <branch>    point repo symlink → worktree (use 'main' for main repo path)
  gwt symlink status                    show current symlink targets

TUI KEYS
  ↑/↓  k/j   navigate
  enter       open in new tmux window
  c           create worktree (prompts for branch)
  d           delete worktree (confirm)
  p           push branch upstream
  s           set symlink → selected worktree
  q           quit

  ⇒ indicator = symlink currently points to this worktree
`)
}

func cmdCreate(cfg *Config, db *DB, repoName, branch string) error {
	repo := cfg.RepoByName(repoName)
	if repo == nil {
		return fmt.Errorf("repo %q not found. run: gwt repo add <path>", repoName)
	}
	dest := filepath.Join(cfg.WorktreeRootAbs(), repoName+"-"+branch)
	if err := os.MkdirAll(cfg.WorktreeRootAbs(), 0755); err != nil {
		return err
	}
	if err := AddWorktree(expandHome(repo.Path), branch, dest); err != nil {
		return err
	}
	if err := db.Insert(Worktree{RepoName: repoName, Branch: branch, Path: dest}); err != nil {
		return err
	}
	if err := Push(expandHome(repo.Path), branch); err != nil {
		fmt.Fprintf(os.Stderr, "warn: push failed: %v\n", err)
	}
	return nil
}

func cmdDelete(cfg *Config, db *DB, repoName, branch string) error {
	repo := cfg.RepoByName(repoName)
	if repo == nil {
		return fmt.Errorf("repo %q not found", repoName)
	}
	dest := filepath.Join(cfg.WorktreeRootAbs(), repoName+"-"+branch)
	_ = RemoveWorktree(expandHome(repo.Path), dest)
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	return db.Delete(repoName, branch)
}

func cmdPush(cfg *Config, repoName, branch string) error {
	repo := cfg.RepoByName(repoName)
	if repo == nil {
		return fmt.Errorf("repo %q not found", repoName)
	}
	return Push(expandHome(repo.Path), branch)
}

func cmdSymlink(cfg *Config, repoName, branch string) error {
	repo := cfg.RepoByName(repoName)
	if repo == nil {
		return fmt.Errorf("repo %q not found", repoName)
	}
	var sourcePath string
	if branch == "main" || branch == "master" {
		sourcePath = expandHome(repo.Path)
	} else {
		sourcePath = filepath.Join(cfg.WorktreeRootAbs(), repoName+"-"+branch)
	}
	return SetSymlink(repo, sourcePath)
}

func detectCurrentRepo(cfg *Config) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for _, r := range cfg.Repos {
		abs := expandHome(r.Path)
		if strings.HasPrefix(cwd, abs) {
			return r.Name
		}
	}
	return ""
}

type tuiWorktree struct {
	w          Worktree
	symlinkSrc string // current symlink dest for this repo
}

func (t tuiWorktree) GetRepoName() string    { return t.w.RepoName }
func (t tuiWorktree) GetBranch() string      { return t.w.Branch }
func (t tuiWorktree) GetPath() string        { return t.w.Path }
func (t tuiWorktree) GetSymlinkStatus() string { return t.symlinkSrc }

type tuiActions struct {
	cfg *Config
	db  *DB
}

func (a *tuiActions) Delete(repoName, branch string) error {
	return cmdDelete(a.cfg, a.db, repoName, branch)
}
func (a *tuiActions) Push(repoName, branch string) error {
	return cmdPush(a.cfg, repoName, branch)
}
func (a *tuiActions) Create(repoName, branch string) (tui.Worktree, error) {
	if err := cmdCreate(a.cfg, a.db, repoName, branch); err != nil {
		return nil, err
	}
	dest := filepath.Join(a.cfg.WorktreeRootAbs(), repoName+"-"+branch)
	repo := a.cfg.RepoByName(repoName)
	sym := ""
	if repo != nil {
		sym = SymlinkStatus(repo)
	}
	return tuiWorktree{
		w:          Worktree{RepoName: repoName, Branch: branch, Path: dest},
		symlinkSrc: sym,
	}, nil
}
func (a *tuiActions) OpenTmux(name, dir string) error {
	return NewTmuxWindow(name, dir)
}
func (a *tuiActions) Symlink(repoName, branch string) error {
	return cmdSymlink(a.cfg, repoName, branch)
}
func (a *tuiActions) RepoNames() []string {
	names := make([]string, len(a.cfg.Repos))
	for i, r := range a.cfg.Repos {
		names[i] = r.Name
	}
	return names
}

func launchTUI(cfg *Config, db *DB) {
	wts, err := db.List()
	if err != nil {
		fatal("%v", err)
	}
	// build symlink status map per repo
	symlinkStatus := map[string]string{}
	for _, r := range cfg.Repos {
		symlinkStatus[r.Name] = SymlinkStatus(&r)
	}
	items := make([]tui.Worktree, len(wts))
	for i, w := range wts {
		items[i] = tuiWorktree{w: w, symlinkSrc: symlinkStatus[w.RepoName]}
	}
	currentRepo := detectCurrentRepo(cfg)
	actions := &tuiActions{cfg: cfg, db: db}
	m := tui.New(items, currentRepo, actions)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fatal("%v", err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gwt: "+format+"\n", args...)
	os.Exit(1)
}
