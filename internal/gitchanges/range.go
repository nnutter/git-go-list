package gitchanges

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// RangeChangedFiles lists files changed in a commit range using only go-git.
// Supported forms:
//
//	A..B   two-dot diff (changes in B not in A)
//	A...B  three-dot diff (merge-base of A and B compared to B)
//	R      shorthand for R..HEAD
//
// Paths are absolute and sorted.
func RangeChangedFiles(startDir, revRange string) ([]string, error) {
	repo, root, err := OpenRepo(startDir)
	if err != nil {
		return nil, err
	}

	fromRev, toRev, threeDot, err := parseRevRange(revRange)
	if err != nil {
		return nil, err
	}

	if threeDot {
		return threeDotFiles(repo, root, fromRev, toRev)
	}

	return twoDotFiles(repo, root, fromRev, toRev)
}

func parseRevRange(revRange string) (from, to string, threeDot bool, err error) {
	revRange = strings.TrimSpace(revRange)
	if revRange == "" {
		return "", "", false, fmt.Errorf("empty revision range")
	}

	if before, after, ok := strings.Cut(revRange, "..."); ok {
		if before == "" || after == "" {
			return "", "", false, fmt.Errorf("invalid range %q: expected <from>...<to>", revRange)
		}

		return strings.TrimSpace(before), strings.TrimSpace(after), true, nil
	}

	if before, after, ok := strings.Cut(revRange, ".."); ok {
		if before == "" || after == "" {
			return "", "", false, fmt.Errorf("invalid range %q: expected <from>..<to>", revRange)
		}

		return strings.TrimSpace(before), strings.TrimSpace(after), false, nil
	}

	return revRange, "HEAD", false, nil
}

func twoDotFiles(repo *git.Repository, root, fromRev, toRev string) ([]string, error) {
	from, err := resolveCommit(repo, fromRev)
	if err != nil {
		return nil, err
	}

	to, err := resolveCommit(repo, toRev)
	if err != nil {
		return nil, err
	}

	return patchFiles(root, from, to)
}

func threeDotFiles(repo *git.Repository, root, fromRev, toRev string) ([]string, error) {
	from, err := resolveCommit(repo, fromRev)
	if err != nil {
		return nil, err
	}

	to, err := resolveCommit(repo, toRev)
	if err != nil {
		return nil, err
	}

	bases, err := from.MergeBase(to)
	if err != nil {
		return nil, fmt.Errorf("find merge-base of %q and %q: %w", fromRev, toRev, err)
	}

	if len(bases) == 0 {
		return nil, fmt.Errorf("no merge-base for %q and %q", fromRev, toRev)
	}

	return patchFiles(root, bases[0], to)
}

// IsRevision reports whether rev resolves to an object in repo.
func IsRevision(repo *git.Repository, rev string) bool {
	_, err := repo.ResolveRevision(plumbing.Revision(rev))

	return err == nil
}

func resolveCommit(repo *git.Repository, rev string) (*object.Commit, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", rev, err)
	}

	for range 10 {
		commit, err := repo.CommitObject(*hash)
		if err == nil {
			return commit, nil
		}

		tag, tagErr := repo.TagObject(*hash)
		if tagErr != nil {
			return nil, fmt.Errorf("resolve %q to commit: %w", rev, err)
		}

		hash = &tag.Target
	}

	return nil, fmt.Errorf("resolve %q: tag chain too deep", rev)
}

func patchFiles(root string, from, to *object.Commit) ([]string, error) {
	if from.Hash == to.Hash {
		return nil, nil
	}

	patch, err := from.Patch(to)
	if err != nil {
		return nil, fmt.Errorf("diff %s vs %s: %w", from.Hash, to.Hash, err)
	}

	seen := make(map[string]struct{})

	for _, fp := range patch.FilePatches() {
		fromFile, toFile := fp.Files()

		if fromFile != nil && fromFile.Path() != "" {
			seen[fromFile.Path()] = struct{}{}
		}

		if toFile != nil && toFile.Path() != "" {
			seen[toFile.Path()] = struct{}{}
		}
	}

	files := make([]string, 0, len(seen))
	for rel := range seen {
		files = append(files, filepath.Join(root, filepath.FromSlash(rel)))
	}

	sort.Strings(files)

	return files, nil
}
