package gitchanges_test

import (
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/nnutter/git-go-list/internal/gitchanges"
)

func commitFile(t *testing.T, root string, worktree *git.Worktree, rel, content, msg string) string {
	t.Helper()

	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))

	_, err := worktree.Add(rel)
	require.NoError(t, err)

	_, err = worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	return abs
}

func headHash(t *testing.T, repo *git.Repository) string {
	t.Helper()

	ref, err := repo.Head()
	require.NoError(t, err)

	return ref.Hash().String()
}

func TestRangeTwoDot(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	repo, err := git.PlainOpen(root)
	require.NoError(t, err)

	commitFile(t, root, worktree, "a.go", "package main\n", "add a")
	base := headHash(t, repo)
	bAbs := commitFile(t, root, worktree, "b.go", "package main\n", "add b")
	head := headHash(t, repo)

	files, err := gitchanges.RangeChangedFiles(root, base+".."+head)
	require.NoError(t, err)
	require.Equal(t, []string{bAbs}, files)
}

func TestRangeSingleRevMeansSinceRev(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	repo, err := git.PlainOpen(root)
	require.NoError(t, err)

	commitFile(t, root, worktree, "a.go", "package main\n", "add a")
	base := headHash(t, repo)
	bAbs := commitFile(t, root, worktree, "b.go", "package main\n", "add b")

	files, err := gitchanges.RangeChangedFiles(root, base)
	require.NoError(t, err)
	require.Equal(t, []string{bAbs}, files)
}

func TestRangeIdenticalRevsEmpty(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	repo, err := git.PlainOpen(root)
	require.NoError(t, err)

	commitFile(t, root, worktree, "a.go", "package main\n", "add a")
	head := headHash(t, repo)

	files, err := gitchanges.RangeChangedFiles(root, head+".."+head)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestRangeThreeDotUsesMergeBase(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)

	commitFile(t, root, worktree, "a.go", "package main\n", "add a")

	// Branch feature from the initial commit.
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("feature"),
		Create: true,
	}))
	cAbs := commitFile(t, root, worktree, "c.go", "package main\n", "add c")

	// Back on master, advance independently.
	require.NoError(t, worktree.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}))
	commitFile(t, root, worktree, "b.go", "package main\n", "add b")

	files, err := gitchanges.RangeChangedFiles(root, "master...feature")
	require.NoError(t, err)
	require.Equal(t, []string{cAbs}, files)
}

func TestRangeInvalid(t *testing.T) {
	t.Parallel()

	root, worktree := initRepo(t)
	commitFile(t, root, worktree, "a.go", "package main\n", "add a")

	for _, revRange := range []string{"", "a..", "..b", "a...", "...b", "does-not-exist..HEAD"} {
		_, err := gitchanges.RangeChangedFiles(root, revRange)
		require.Error(t, err, "range %q", revRange)
	}
}
