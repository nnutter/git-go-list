package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/nnutter/git-go-list/internal/cli"
)

func initCLIRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module example.com/demo\n\ngo 1.27.0\n"),
		0o644,
	))

	repo, err := git.PlainInit(root, false)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	abs := filepath.Join(root, "pkg", "foo", "foo.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte("package foo\n"), 0o644))

	_, err = worktree.Add(".")
	require.NoError(t, err)

	_, err = worktree.Commit("init", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(abs, []byte("package foo\n\n// change\n"), 0o644))

	return root
}

func runRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := cli.NewRoot()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return buf.String(), err
}

func TestRootDefaultImportPath(t *testing.T) {
	t.Parallel()

	root := initCLIRepo(t)

	out, err := runRoot(t, "-C", root)
	require.NoError(t, err)
	require.Equal(t, "example.com/demo/pkg/foo\n", out)
}

func TestRootRelativeAndDirs(t *testing.T) {
	t.Parallel()

	root := initCLIRepo(t)

	out, err := runRoot(t, "-C", root, "--relative")
	require.NoError(t, err)
	require.Equal(t, "pkg/foo\n", out)

	out, err = runRoot(t, "-C", root, "--dirs")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "pkg", "foo")+"\n", out)
}

func TestRootMutuallyExclusive(t *testing.T) {
	t.Parallel()

	root := initCLIRepo(t)

	_, err := runRoot(t, "-C", root, "--staged", "--range", "HEAD")
	require.Error(t, err)

	_, err = runRoot(t, "-C", root, "--json", "--relative")
	require.Error(t, err)
}

func TestRootPositionalRange(t *testing.T) {
	t.Parallel()

	root := initCLIRepo(t)

	// Commit the dirty worktree change so HEAD~1..HEAD covers it.
	repo, err := git.PlainOpen(root)
	require.NoError(t, err)

	worktree, err := repo.Worktree()
	require.NoError(t, err)

	_, err = worktree.Add(".")
	require.NoError(t, err)

	_, err = worktree.Commit("change foo", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@example.com"},
	})
	require.NoError(t, err)

	out, err := runRoot(t, "-C", root, "HEAD~1..HEAD")
	require.NoError(t, err)
	require.Equal(t, "example.com/demo/pkg/foo\n", out)

	// Bare revisions work positionally too (meaning <rev>..HEAD).
	out, err = runRoot(t, "-C", root, "HEAD~1")
	require.NoError(t, err)
	require.Equal(t, "example.com/demo/pkg/foo\n", out)

	// A positional range plus pathspec narrows further.
	out, err = runRoot(t, "-C", root, "HEAD~1..HEAD", "other")
	require.NoError(t, err)
	require.Empty(t, out)
}
