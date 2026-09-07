package gitchanges_test

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/nnutter/git-go-list/internal/gitchanges"
)

func initRepo(t *testing.T) (string, *git.Worktree) {
	t.Helper()

	root := t.TempDir()

	repo, err := git.PlainInit(root, false)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	return root, worktree
}

func writeFile(t *testing.T, root, rel, content string) string {
	t.Helper()

	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))

	return abs
}

func commitAll(t *testing.T, worktree *git.Worktree, msg string) {
	t.Helper()

	_, err := worktree.Add(".")
	require.NoError(t, err)
	_, err = worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)
}

func TestWorktreeUntrackedIncludedByDefault(t *testing.T) {
	t.Parallel()

	root, _ := initRepo(t)
	abs := writeFile(t, root, "main.go", "package main\n")

	files, err := gitchanges.WorktreeChangedFiles(root, false)
	require.NoError(t, err)
	require.Contains(t, files, abs)

	staged, err := gitchanges.WorktreeChangedFiles(root, true)
	require.NoError(t, err)
	require.NotContains(t, staged, abs)
}

func TestWorktreeStagedFile(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	abs := writeFile(t, root, "main.go", "package main\n")
	_, err := worktree.Add("main.go")
	require.NoError(t, err)

	all, err := gitchanges.WorktreeChangedFiles(root, false)
	require.NoError(t, err)
	require.Contains(t, all, abs)

	staged, err := gitchanges.WorktreeChangedFiles(root, true)
	require.NoError(t, err)
	require.Contains(t, staged, abs)
}

func TestWorktreeUnstagedModification(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	abs := writeFile(t, root, "main.go", "package main\n")
	commitAll(t, worktree, "init")

	require.NoError(t, os.WriteFile(abs, []byte("package main\n\n// change\n"), 0o644))

	all, err := gitchanges.WorktreeChangedFiles(root, false)
	require.NoError(t, err)
	require.Contains(t, all, abs)

	staged, err := gitchanges.WorktreeChangedFiles(root, true)
	require.NoError(t, err)
	require.NotContains(t, staged, abs)
}

func TestWorktreeClean(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	writeFile(t, root, "main.go", "package main\n")
	commitAll(t, worktree, "init")

	files, err := gitchanges.WorktreeChangedFiles(root, false)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestWorktreeFromSubdir(t *testing.T) {
	t.Parallel()

	root, _ := initRepo(t)
	abs := writeFile(t, root, "pkg/foo/foo.go", "package foo\n")
	sub := filepath.Join(root, "pkg", "foo")

	files, err := gitchanges.WorktreeChangedFiles(sub, false)
	require.NoError(t, err)
	require.Contains(t, files, abs)
}
