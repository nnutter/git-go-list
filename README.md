# git-go-list

`git-go-list` mimics `go list` for changed code: it inspects staged,
working tree, or commit-range changes with
[go-git](https://github.com/go-git/go-git) (no `git` CLI) and prints the
changed Go packages in the current module.

## Install

```sh
go install github.com/nnutter/git-go-list/cmd/git-go-list@latest
```

## Use

```sh
# Worktree vs HEAD (staged + unstaged + untracked .go files)
git-go-list

# Staged only (HEAD vs index)
git-go-list --staged

# Commit ranges (two-dot, three-dot via merge-base, or <rev> == <rev>..HEAD)
git-go-list --range 'HEAD~3..HEAD'
git-go-list --range 'main...HEAD'
git-go-list --range 'abc123'

# The range can also be positional, git-diff style
# (first arg with '..' or resolving as a revision, unless it's a file)
git-go-list 'HEAD~3..HEAD'
git-go-list main...HEAD -- pkg/internal

# Only packages under a pathspec
git-go-list -- pkg/internal

# Test / vet only what changed
go test $(git-go-list)
git-go-list -0 | xargs -0 go test
git-go-list --staged | xargs go vet
```

## Output

Default output is one sorted import path per line, like `go list`:

```sh
$ git-go-list
github.com/nnutter/git-go-list/internal/cli
github.com/nnutter/git-go-list/internal/packages
```

Other formats:

```sh
git-go-list --relative            # pkg/foo instead of full import path
git-go-list --dirs                # absolute directories
git-go-list -f '{{.ImportPath}} {{.Dir}}'
git-go-list --json
git-go-list -0                    # NUL delimiters for xargs -0
```

## Behavior

- Module discovery walks up for `go.mod`; only files in that module count.
- `vendor/`, nested modules, and files outside the module are skipped.
- Only `*.go` files count by default (including `*_test.go`).
  Use `--all-files` to also count non-Go files inside dirs that contain
  Go files (embedded assets, CGO, etc.).
- Deleted packages are still reported by derived import path.
- `--staged` and `--range` are mutually exclusive.
- `-e/--tolerate-errors` warns instead of failing, like `go list -e`.

## Development

```sh
mise run tests    # go test -cover ./...
mise run fixers   # gofumpt, goimports, go fix, tidy, testifylint
mise run linters  # vet, staticcheck, errcheck, gosec, etc.
```
