package packages_test

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/nnutter/git-go-list/internal/packages"
)

func initModuleRepo(t *testing.T, modulePath string) (string, *git.Worktree) {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module "+modulePath+"\n\ngo 1.27.0\n"),
		0o644,
	))

	repo, err := git.PlainInit(root, false)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	return root, worktree
}

func writeAndCommit(t *testing.T, root string, worktree *git.Worktree, rel, content, msg string) {
	t.Helper()

	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))

	_, err := worktree.Add(".")
	require.NoError(t, err)

	_, err = worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)
}

func writeOnly(t *testing.T, root, rel, content string) {
	t.Helper()

	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

func TestCollectWorktreeGoChange(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "pkg/foo/foo.go", "package foo\n", "init")
	writeOnly(t, root, "pkg/foo/foo.go", "package foo\n\n// change\n")

	pkgs, err := packages.Collect(packages.Options{Dir: root})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Equal(t, "example.com/demo/pkg/foo", pkgs[0].ImportPath)
	require.Equal(t, "pkg/foo", pkgs[0].Rel)
}

func TestCollectIgnoresNonGoByDefault(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "pkg/foo/foo.go", "package foo\n", "init")
	writeOnly(t, root, "pkg/foo/README.md", "# docs\n")

	pkgs, err := packages.Collect(packages.Options{Dir: root})
	require.NoError(t, err)
	require.Empty(t, pkgs)

	pkgs, err = packages.Collect(packages.Options{Dir: root, AllFiles: true})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Equal(t, "example.com/demo/pkg/foo", pkgs[0].ImportPath)
}

func TestCollectStagedOnly(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "pkg/foo/foo.go", "package foo\n", "init")
	writeOnly(t, root, "pkg/foo/foo.go", "package foo\n\n// unstaged\n")

	pkgs, err := packages.Collect(packages.Options{Dir: root, StagedOnly: true})
	require.NoError(t, err)
	require.Empty(t, pkgs)

	_, err = worktree.Add("pkg/foo/foo.go")
	require.NoError(t, err)

	pkgs, err = packages.Collect(packages.Options{Dir: root, StagedOnly: true})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
}

func TestCollectRange(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "a.go", "package demo\n", "add a")

	repo, err := git.PlainOpen(root)
	require.NoError(t, err)

	baseRef, err := repo.Head()
	require.NoError(t, err)
	base := baseRef.Hash().String()

	writeAndCommit(t, root, worktree, "pkg/bar/bar.go", "package bar\n", "add bar")

	pkgs, err := packages.Collect(packages.Options{Dir: root, RevRange: base + "..HEAD"})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Equal(t, "example.com/demo/pkg/bar", pkgs[0].ImportPath)
}

func TestCollectPathspec(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "a.go", "package demo\n", "init")
	writeOnly(t, root, "pkg/foo/foo.go", "package foo\n")
	writeOnly(t, root, "pkg/bar/bar.go", "package bar\n")

	pkgs, err := packages.Collect(packages.Options{Dir: root, Pathspecs: []string{"pkg/foo"}})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	require.Equal(t, "example.com/demo/pkg/foo", pkgs[0].ImportPath)
}

func TestCollectDedupesAndSorts(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "a.go", "package demo\n", "init")
	writeOnly(t, root, "pkg/b/b.go", "package b\n")
	writeOnly(t, root, "pkg/a/a.go", "package a\n")
	writeOnly(t, root, "pkg/a/a2.go", "package a\n")

	pkgs, err := packages.Collect(packages.Options{Dir: root})
	require.NoError(t, err)
	require.Len(t, pkgs, 2)
	require.Equal(t, "example.com/demo/pkg/a", pkgs[0].ImportPath)
	require.Equal(t, "example.com/demo/pkg/b", pkgs[1].ImportPath)
}

func TestSplitArgs(t *testing.T) {
	t.Parallel()

	root, worktree := initModuleRepo(t, "example.com/demo")
	writeAndCommit(t, root, worktree, "a.go", "package demo\n", "init")
	writeOnly(t, root, "pkg/foo/foo.go", "package foo\n")

	// Explicit flag wins; everything else stays a pathspec.
	rng, specs := packages.SplitArgs(root, "HEAD", []string{"pkg"})
	require.Equal(t, "HEAD", rng)
	require.Equal(t, []string{"pkg"}, specs)

	// Positional two-dot range with trailing pathspec.
	rng, specs = packages.SplitArgs(root, "", []string{"HEAD~1..HEAD", "pkg"})
	require.Equal(t, "HEAD~1..HEAD", rng)
	require.Equal(t, []string{"pkg"}, specs)

	// Existing files always win over range interpretation.
	rng, specs = packages.SplitArgs(root, "", []string{"pkg"})
	require.Empty(t, rng)
	require.Equal(t, []string{"pkg"}, specs)

	// Bare revisions resolve positionally.
	rng, specs = packages.SplitArgs(root, "", []string{"HEAD"})
	require.Equal(t, "HEAD", rng)
	require.Empty(t, specs)

	// Unknown names stay pathspecs.
	rng, specs = packages.SplitArgs(root, "", []string{"nosuchfile"})
	require.Empty(t, rng)
	require.Equal(t, []string{"nosuchfile"}, specs)

	// Parent traversals are paths, not ranges.
	rng, specs = packages.SplitArgs(root, "", []string{"../foo"})
	require.Empty(t, rng)
	require.Equal(t, []string{"../foo"}, specs)

	// No args means worktree mode.
	rng, specs = packages.SplitArgs(root, "", nil)
	require.Empty(t, rng)
	require.Empty(t, specs)
}
