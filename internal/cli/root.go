package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/nnutter/git-go-list/internal/packages"
)

// NewRoot builds the git-go-list command.
func NewRoot() *cobra.Command {
	var (
		changeDir string
		staged    bool
		cached    bool
		revRange  string
		format    string
		asJSON    bool
		relative  bool
		showDirs  bool
		zero      bool
		tolerate  bool
		allFiles  bool
	)

	cmd := &cobra.Command{
		Use:   "git-go-list [flags] [range] [-- pathspec...]",
		Short: "List changed Go packages in the current module",
		Long: `List Go packages with changed files, mimicking 'go list'.

By default it diffs HEAD against the working tree (staged, unstaged,
and untracked files) using go-git, without shelling out to git.
Use --staged for HEAD vs index only, or --range for commit ranges.

The range can also be given positionally, git-diff style: the first
argument is treated as a range when it contains '..' or resolves as
a revision, unless it exists on disk. Pass --range to override.

Examples:
  git-go-list
  git-go-list --staged
  git-go-list --range 'main...HEAD'
  git-go-list main...HEAD -- pkg/internal
  go test $(git-go-list)
  git-go-list -0 | xargs -0 go test`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if staged && revRange != "" || cached && revRange != "" {
				return fmt.Errorf("--staged and --range are mutually exclusive")
			}

			if format != "" && (asJSON || relative || showDirs) {
				return fmt.Errorf("--format cannot be combined with --json, --relative, or --dirs")
			}

			if asJSON && (relative || showDirs) {
				return fmt.Errorf("--json cannot be combined with --relative or --dirs")
			}

			if relative && showDirs {
				return fmt.Errorf("--relative and --dirs are mutually exclusive")
			}

			if zero && asJSON {
				return fmt.Errorf("--zero cannot be combined with --json")
			}

			rng, pathspecs := packages.SplitArgs(changeDir, revRange, args)

			pkgs, err := packages.Collect(packages.Options{
				Dir:        changeDir,
				StagedOnly: staged || cached,
				RevRange:   rng,
				AllFiles:   allFiles,
				Pathspecs:  pathspecs,
			})
			if err != nil {
				if tolerate {
					fmt.Fprintf(os.Stderr, "git-go-list: %v\n", err)

					return nil
				}

				return err
			}

			out := cmd.OutOrStdout()

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")

				return enc.Encode(pkgs)
			}

			lines := make([]string, 0, len(pkgs))

			switch {
			case format != "":
				tmpl, err := template.New("format").Parse(format)
				if err != nil {
					return fmt.Errorf("parse --format: %w", err)
				}

				var sb strings.Builder

				for _, pkg := range pkgs {
					sb.Reset()

					if err := tmpl.Execute(&sb, pkg); err != nil {
						return fmt.Errorf("execute --format: %w", err)
					}

					lines = append(lines, sb.String())
				}
			case relative:
				for _, pkg := range pkgs {
					lines = append(lines, pkg.Rel)
				}
			case showDirs:
				for _, pkg := range pkgs {
					lines = append(lines, pkg.Dir)
				}
			default:
				for _, pkg := range pkgs {
					lines = append(lines, pkg.ImportPath)
				}
			}

			delim := "\n"
			if zero {
				delim = "\x00"
			}

			for _, line := range lines {
				if _, err := fmt.Fprint(out, line+delim); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&changeDir, "directory", "C", ".", "Run as if in directory")
	cmd.Flags().BoolVar(&staged, "staged", false, "List staged changes only (HEAD vs index)")
	cmd.Flags().BoolVar(&cached, "cached", false, "Alias for --staged (like git diff --cached)")
	cmd.Flags().StringVar(&revRange, "range", "", "Commit range: <a>..<b>, <a>...<b>, or <rev> (= <rev>..HEAD)")
	cmd.Flags().StringVarP(&format, "format", "f", "", "Go template per package (fields: ImportPath Dir Rel Module)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON array of packages")
	cmd.Flags().BoolVar(&relative, "relative", false, "Print module-relative paths instead of import paths")
	cmd.Flags().BoolVar(&showDirs, "dirs", false, "Print absolute package directories instead of import paths")
	cmd.Flags().BoolVarP(&zero, "zero", "0", false, "Use NUL delimiters instead of newlines")
	cmd.Flags().BoolVarP(&tolerate, "tolerate-errors", "e", false, "Warn instead of failing on errors (like go list -e)")
	cmd.Flags().BoolVar(&allFiles, "all-files", false, "Count non-Go files in Go package dirs as changes")

	return cmd
}
