package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func AddWorktree(repoPath, branch, dest string) error {
	// try creating new branch from HEAD first
	_, err := gitCmd(repoPath, "worktree", "add", "-b", branch, dest, "HEAD")
	if err != nil {
		// branch may already exist — check out existing branch
		_, err = gitCmd(repoPath, "worktree", "add", dest, branch)
	}
	return err
}

func RemoveWorktree(repoPath, wtPath string) error {
	_, err := gitCmd(repoPath, "worktree", "remove", "--force", wtPath)
	return err
}

func Push(repoPath, branch string) error {
	_, err := gitCmd(repoPath, "push", "-u", "origin", branch)
	return err
}

func GetRemoteURL(repoPath string) (string, error) {
	return gitCmd(repoPath, "remote", "get-url", "origin")
}
