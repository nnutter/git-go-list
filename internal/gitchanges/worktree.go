package gitchanges

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	git "github.com/go-git/go-git/v5"
)

// OpenRepo opens the Git repository containing startDir by walking up
// for a .git file or directory. It returns the repository and its
// worktree root.
func OpenRepo(startDir string) (*git.Repository, string, error) {
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve %q: %w", startDir, err)
	}

	st, err := os.Stat(abs)
	if err == nil && !st.IsDir() {
		abs = filepath.Dir(abs)
	}

	dir := filepath.Clean(abs)

	for {
		candidate := filepath.Join(dir, ".git")
		if st, err := os.Stat(candidate); err == nil {
			if st.IsDir() || !st.IsDir() {
				// EnableDotGitCommonDir follows the commondir pointer
				// so linked worktrees see refs (e.g. origin/*) and
				// objects stored in the common directory.
				repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{
					EnableDotGitCommonDir: true,
				})
				if err != nil {
					return nil, "", fmt.Errorf("open repo at %s: %w", dir, err)
				}

				return repo, dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, "", fmt.Errorf("no git repository found from %q", startDir)
		}

		dir = parent
	}
}

// WorktreeChangedFiles lists changed files in the working tree using
// go-git status. When stagedOnly is true only HEAD-vs-index changes are
// returned; otherwise staged, unstaged, and untracked files are returned.
// Paths are absolute and sorted.
func WorktreeChangedFiles(startDir string, stagedOnly bool) ([]string, error) {
	repo, root, err := OpenRepo(startDir)
	if err != nil {
		return nil, err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("load worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("read git status: %w", err)
	}

	var relPaths []string

	for rel, fileStatus := range status {
		if fileStatus == nil {
			continue
		}

		if stagedOnly {
			if fileStatus.Staging != git.Unmodified && fileStatus.Staging != git.Untracked {
				relPaths = append(relPaths, rel)
			}

			continue
		}

		staged := fileStatus.Staging != git.Unmodified && fileStatus.Staging != git.Untracked
		unstaged := fileStatus.Worktree != git.Unmodified && fileStatus.Worktree != git.Untracked
		untracked := fileStatus.Staging == git.Untracked || fileStatus.Worktree == git.Untracked

		if staged || unstaged || untracked {
			relPaths = append(relPaths, rel)
		}
	}

	files, err := expandDirs(root, relPaths)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	return files, nil
}

func expandDirs(root string, relPaths []string) ([]string, error) {
	var files []string

	for _, rel := range relPaths {
		abs := filepath.Join(root, filepath.FromSlash(rel))

		st, err := os.Stat(abs)
		if err != nil {
			// Deleted worktree file: still report the path so callers
			// can derive its package.
			files = append(files, abs)

			continue
		}

		if !st.IsDir() {
			files = append(files, abs)

			continue
		}

		err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !entry.IsDir() {
				files = append(files, path)
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("expand directory %s: %w", abs, err)
		}
	}

	return files, nil
}
