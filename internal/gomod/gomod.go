package gomod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// Info describes the Go module containing a directory.
type Info struct {
	// Dir is the absolute module root (directory containing go.mod).
	Dir string
	// Path is the module path declared in go.mod.
	Path string
}

// Find walks up from start (file or directory) to locate go.mod.
func Find(start string) (Info, error) {
	dir, err := absDir(start)
	if err != nil {
		return Info{}, err
	}

	for {
		candidate := filepath.Join(dir, "go.mod")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			path, err := parseModulePath(candidate)
			if err != nil {
				return Info{}, err
			}

			return Info{Dir: dir, Path: path}, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return Info{}, fmt.Errorf("no go.mod found from %q", start)
		}

		dir = parent
	}
}

func absDir(start string) (string, error) {
	if start == "" {
		return "", errors.New("empty start directory")
	}

	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", start, err)
	}

	st, err := os.Stat(abs)
	if err == nil && !st.IsDir() {
		return filepath.Dir(abs), nil
	}

	return filepath.Clean(abs), nil
}

func parseModulePath(goMod string) (string, error) {
	data, err := os.ReadFile(goMod)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", goMod, err)
	}

	f, err := modfile.Parse(goMod, data, nil)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", goMod, err)
	}

	if f.Module == nil || f.Module.Mod.Path == "" {
		return "", fmt.Errorf("missing module directive in %s", goMod)
	}

	return f.Module.Mod.Path, nil
}

// Package describes a Go package directory within a module.
type Package struct {
	// ImportPath is the full import path (module path + relative dir).
	ImportPath string
	// Dir is the absolute package directory.
	Dir string
	// Rel is the slash-separated path relative to the module root.
	// "." denotes the module root.
	Rel string
	// Module is the module path.
	Module string
}

// PackageForFile maps an absolute or start-relative file path to its
// containing package. It reports ok=false for files outside the module,
// inside vendor, or inside a nested module.
func PackageForFile(mod Info, filePath string) (pkg Package, ok bool, err error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return Package{}, false, fmt.Errorf("resolve %q: %w", filePath, err)
	}

	pkgDir := filepath.Dir(abs)
	dirRel, ok := relWithinModule(mod.Dir, pkgDir)
	if !ok {
		return Package{}, false, nil
	}

	return packageForDirRel(mod, pkgDir, dirRel)
}

// PackageForDir maps a package directory to its package description.
// It reports ok=false for directories outside the module, vendor, or
// nested modules.
func PackageForDir(mod Info, dirPath string) (pkg Package, ok bool, err error) {
	abs, err := filepath.Abs(dirPath)
	if err != nil {
		return Package{}, false, fmt.Errorf("resolve %q: %w", dirPath, err)
	}

	st, err := os.Stat(abs)
	if err == nil && !st.IsDir() {
		abs = filepath.Dir(abs)
	}

	rel, ok := relWithinModule(mod.Dir, abs)
	if !ok {
		return Package{}, false, nil
	}

	return packageForDirRel(mod, abs, rel)
}

func packageForDirRel(mod Info, absDir, dirRel string) (Package, bool, error) {
	slashRel := filepath.ToSlash(dirRel)

	if slashRel == "." {
		return Package{
			ImportPath: mod.Path,
			Dir:        filepath.Clean(absDir),
			Rel:        ".",
			Module:     mod.Path,
		}, true, nil
	}

	if slashRel == "vendor" || strings.HasPrefix(slashRel, "vendor/") {
		return Package{}, false, nil
	}

	nested, err := isNestedModule(mod.Dir, absDir)
	if err != nil {
		return Package{}, false, err
	}

	if nested {
		return Package{}, false, nil
	}

	return Package{
		ImportPath: mod.Path + "/" + slashRel,
		Dir:        filepath.Clean(absDir),
		Rel:        slashRel,
		Module:     mod.Path,
	}, true, nil
}

func relWithinModule(root, abs string) (string, bool) {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", false
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}

	return rel, true
}

// isNestedModule reports whether dir (or any parent up to but excluding
// module root) contains its own go.mod.
func isNestedModule(moduleRoot, dir string) (bool, error) {
	root := filepath.Clean(moduleRoot)
	current := filepath.Clean(dir)

	for {
		if current != root {
			candidate := filepath.Join(current, "go.mod")
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return true, nil
			}
		}

		if current == root {
			return false, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}

		// If dir escaped the module root, stop; caller already ensures
		// containment but guard anyway.
		rel, err := filepath.Rel(root, parent)
		if err == nil && (rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return false, nil
		}

		current = parent
	}
}
