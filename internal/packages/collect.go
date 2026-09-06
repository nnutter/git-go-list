package packages

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nnutter/git-go-list/internal/gitchanges"
	"github.com/nnutter/git-go-list/internal/gomod"
)

// Options selects which changes to list.
type Options struct {
	// Dir is the directory to run in (module and repo discovery start here).
	Dir string
	// StagedOnly lists HEAD-vs-index changes only.
	StagedOnly bool
	// RevRange lists a commit range instead of the worktree.
	// See gitchanges.RangeChangedFiles for syntax.
	RevRange string
	// AllFiles also lists packages whose non-Go files changed, provided
	// the package directory contains Go files.
	AllFiles bool
	// Pathspecs optionally limits results to files under these paths.
	Pathspecs []string
}

// Collect returns sorted, de-duplicated packages with changes.
func Collect(opts Options) ([]gomod.Package, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}

	mod, err := gomod.Find(dir)
	if err != nil {
		return nil, err
	}

	var files []string

	if opts.RevRange != "" {
		files, err = gitchanges.RangeChangedFiles(dir, opts.RevRange)
	} else {
		files, err = gitchanges.WorktreeChangedFiles(dir, opts.StagedOnly)
	}

	if err != nil {
		return nil, err
	}

	specs, err := resolvePathspecs(dir, opts.Pathspecs)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]gomod.Package)

	for _, file := range files {
		if !matchPathspec(file, specs) {
			continue
		}

		if !opts.AllFiles && !strings.HasSuffix(file, ".go") {
			continue
		}

		pkg, ok, err := gomod.PackageForFile(mod, file)
		if err != nil {
			return nil, err
		}

		if !ok {
			continue
		}

		if opts.AllFiles && !strings.HasSuffix(file, ".go") {
			hasGo, err := dirHasGoFiles(pkg.Dir)
			if err != nil {
				return nil, err
			}

			if !hasGo {
				continue
			}
		}

		seen[pkg.ImportPath] = pkg
	}

	pkgs := make([]gomod.Package, 0, len(seen))
	for _, pkg := range seen {
		pkgs = append(pkgs, pkg)
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })

	return pkgs, nil
}

// SplitArgs separates an optional leading revision range from pathspecs.
//
// An explicit revFlag always wins and leaves all args as pathspecs.
// Otherwise the first arg is treated as a range when it looks like one
// (contains ".." outside of path traversals) or resolves as a revision,
// provided it does not exist on disk — existing files always win so
// pathspecs keep working. Use --range to force either interpretation.
func SplitArgs(dir, revFlag string, args []string) (revRange string, pathspecs []string) {
	if revFlag != "" {
		return revFlag, args
	}

	if len(args) == 0 {
		return "", nil
	}

	if !isRangeArg(dir, args[0]) {
		return "", args
	}

	return args[0], args[1:]
}

func isRangeArg(dir, arg string) bool {
	if arg == "" {
		return false
	}

	path := arg
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}

	if _, err := os.Stat(path); err == nil {
		return false
	}

	if looksLikeRange(arg) {
		return true
	}

	repo, _, err := gitchanges.OpenRepo(dir)
	if err != nil {
		return false
	}

	return gitchanges.IsRevision(repo, arg)
}

// looksLikeRange reports whether arg has ".." range syntax rather than
// path traversal syntax ("../" or "/..").
func looksLikeRange(arg string) bool {
	if !strings.Contains(arg, "..") {
		return false
	}

	if strings.Contains(arg, "../") || strings.Contains(arg, "/..") {
		return false
	}

	return arg != ".."
}

func resolvePathspecs(dir string, specs []string) ([]string, error) {
	if len(specs) == 0 {
		return nil, nil
	}

	abs := make([]string, 0, len(specs))

	for _, spec := range specs {
		if spec == "" {
			continue
		}

		if !filepath.IsAbs(spec) {
			spec = filepath.Join(dir, spec)
		}

		clean, err := filepath.Abs(spec)
		if err != nil {
			return nil, err
		}

		abs = append(abs, filepath.Clean(clean))
	}

	return abs, nil
}

func matchPathspec(file string, specs []string) bool {
	if len(specs) == 0 {
		return true
	}

	clean := filepath.Clean(file)

	for _, spec := range specs {
		if clean == spec {
			return true
		}

		if strings.HasPrefix(clean, spec+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

func dirHasGoFiles(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Deleted directory: fall back to true for .go files (handled by
		// caller) and false here is unreachable since non-Go deleted files
		// have no package to report. Treat as no Go files.
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}

	return false, nil
}
