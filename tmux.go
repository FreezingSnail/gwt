package main

import (
	"fmt"
	"os/exec"
)

func NewTmuxWindow(name, dir string) error {
	cmd := exec.Command("tmux", "new-window", "-n", name, "-c", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux: %s", string(out))
	}
	return nil
}
