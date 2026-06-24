package main

import (
	"fmt"
	"os"
)

// SetSymlink points repo's symlink_target at the given source path.
// Always replaces existing symlink/file at target.
func SetSymlink(repo *Repo, sourcePath string) error {
	target := expandHome(repo.SymlinkTarget)
	if target == "" {
		return fmt.Errorf("repo %q has no symlink_target configured", repo.Name)
	}
	// remove existing (symlink, file, or empty dir)
	_ = os.Remove(target)
	return os.Symlink(sourcePath, target)
}

// SymlinkStatus returns what the symlink currently points to, or "" if unset/broken.
func SymlinkStatus(repo *Repo) string {
	if repo.SymlinkTarget == "" {
		return ""
	}
	target := expandHome(repo.SymlinkTarget)
	dest, err := os.Readlink(target)
	if err != nil {
		return ""
	}
	return dest
}
