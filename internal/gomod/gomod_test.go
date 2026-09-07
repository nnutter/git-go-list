package gomod_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nnutter/git-go-list/internal/gomod"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestFindModule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.27.0\n")
	sub := filepath.Join(root, "internal", "foo")
	require.NoError(t, os.MkdirAll(sub, 0o755))

	info, err := gomod.Find(sub)
	require.NoError(t, err)
	require.Equal(t, root, info.Dir)
	require.Equal(t, "example.com/demo", info.Path)
}

func TestFindModuleFromFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.27.0\n")
	file := filepath.Join(root, "main.go")
	writeFile(t, file, "package main\n")

	info, err := gomod.Find(file)
	require.NoError(t, err)
	require.Equal(t, "example.com/demo", info.Path)
}

func TestFindModuleMissing(t *testing.T) {
	t.Parallel()

	_, err := gomod.Find(t.TempDir())
	// Temp dirs live under /tmp which has no go.mod parents in test env,
	// but guard against walking up to /.
	if err == nil {
		t.Skip("found parent go.mod, skipping")
	}

	require.Error(t, err)
}

func TestPackageForFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.27.0\n")

	cases := map[string]struct {
		file       string
		importPath string
		rel        string
		ok         bool
	}{
		"root":          {filepath.Join(root, "main.go"), "example.com/demo", ".", true},
		"subpackage":    {filepath.Join(root, "internal", "foo", "foo.go"), "example.com/demo/internal/foo", "internal/foo", true},
		"vendor":        {filepath.Join(root, "vendor", "x", "x.go"), "", "", false},
		"vendor top":    {filepath.Join(root, "vendor.go"), "example.com/demo", ".", true},
		"outside":       {filepath.Join(root, "..", "other.go"), "", "", false},
		"nested module": {filepath.Join(root, "nested", "n.go"), "", "", false},
	}

	writeFile(t, filepath.Join(root, "nested", "go.mod"), "module example.com/nested\n\ngo 1.27.0\n")

	mod, err := gomod.Find(root)
	require.NoError(t, err)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pkg, ok, err := gomod.PackageForFile(mod, tc.file)
			require.NoError(t, err)
			require.Equal(t, tc.ok, ok)

			if tc.ok {
				require.Equal(t, tc.importPath, pkg.ImportPath)
				require.Equal(t, tc.rel, pkg.Rel)
				require.Equal(t, "example.com/demo", pkg.Module)
			}
		})
	}
}

func TestPackageForDirDeletedStillMaps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.27.0\n")

	mod, err := gomod.Find(root)
	require.NoError(t, err)

	// Directory does not exist (e.g. deleted package); mapping is still derived.
	pkg, ok, err := gomod.PackageForDir(mod, filepath.Join(root, "gone", "pkg"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "example.com/demo/gone/pkg", pkg.ImportPath)
	require.Equal(t, "gone/pkg", pkg.Rel)
}
