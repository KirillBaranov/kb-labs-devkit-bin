package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// initTemplate is built via concatenation to avoid a raw backtick inside a
// Go raw string literal (which cannot contain a backtick character).
var initTemplate = strings.Join([]string{
	"version: 1",
	"",
	"# ── Workspace ─────────────────────────────────────────────────────────────────",
	"workspace:",
	"  packageManager: pnpm   # pnpm | npm | yarn",
	"  # maxDepth: 3          # how deep to scan for packages (default: 5)",
	"",
	"# ── Tasks ─────────────────────────────────────────────────────────────────────",
	"# Define tasks once; kb-devkit runs them across all packages in dep order.",
	"# Cache key = SHA256(inputs). Identical inputs → restore outputs in ~1ms.",
	"#",
	"# Dep syntax:",
	`#   "^build"  — run build for every workspace dependency first (like Turbo ^)`,
	`#   "build"   — run build for the same package first`,
	"#",
	"# tasks:",
	"#   build:",
	`#     command: tsup`,
	`#     inputs:  ["src/**", "tsup.config.ts", "tsconfig*.json"]`,
	`#     outputs: ["dist/**"]`,
	`#     deps:    ["^build"]`,
	"#",
	"#   lint:",
	"#     command: eslint src/",
	`#     inputs:  ["src/**", "eslint.config.*"]`,
	"#     outputs: []",
	"#",
	"#   type-check:",
	"#     command: tsc --noEmit",
	`#     inputs:  ["src/**", "tsconfig*.json"]`,
	"#     outputs: []",
	`#     deps:    ["^build"]`,
	"#",
	"#   test:",
	"#     command: vitest run --passWithNoTests",
	`#     inputs:  ["src/**", "test/**", "vitest.config.*"]`,
	`#     outputs: ["coverage/**"]`,
	`#     deps:    ["build"]`,
	"#",
	"#   deploy:",
	"#     command: ./scripts/deploy.sh",
	`#     inputs:  ["dist/**"]`,
	"#     cache:   false   # always runs — never restored from cache",
	"",
	"# ── Affected detection ────────────────────────────────────────────────────────",
	"# Controls which packages --affected considers changed.",
	"#",
	"# affected:",
	"#   strategy: git         # git | submodules | command",
	"#   #   git        — git diff --name-only HEAD from workspace root",
	"#   #   submodules — walks .gitmodules, diffs inside each submodule",
	"#   #   command    — runs a custom script, reads file paths from stdout",
	"#   # command: ./scripts/changed-files.sh",
	"",
	"# ── Quality presets ───────────────────────────────────────────────────────────",
	"# Presets define rules checked by `kb-devkit check`.",
	"# Packages matched by workspace.categories inherit their preset rules.",
	"#",
	"# presets:",
	"#   node-lib:",
	"#     language: typescript",
	"#     package_json:",
	"#       required_scripts: [build, dev, test, lint, type-check, clean]",
	"#       required_fields:  [name, version, type, engines, exports]",
	"#     tsconfig:",
	`#       must_extend_pattern: "@kb-labs/devkit/tsconfig/"`,
	"#     eslint:",
	"#       must_use_devkit_preset: true",
	"#     structure:",
	`#       required_files: ["src/index.ts", "README.md"]`,
	"",
}, "\n")

var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter devkit.yaml in the current directory",
	Long: `Writes a devkit.yaml with all sections commented out as examples.
Uncomment and adjust the tasks you need — then run kb-devkit run build.

If devkit.yaml already exists, the command exits with an error unless --force is passed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot determine working directory: %w", err)
		}

		dest := filepath.Join(dir, "devkit.yaml")

		if !initForce {
			if _, err := os.Stat(dest); err == nil {
				return fmt.Errorf("devkit.yaml already exists (use --force to overwrite)")
			}
		}

		if err := os.WriteFile(dest, []byte(initTemplate), 0o644); err != nil {
			return fmt.Errorf("write devkit.yaml: %w", err)
		}

		o := newOutput()
		o.OK("Created devkit.yaml")
		fmt.Println()
		fmt.Println("  Uncomment the tasks you need, then:")
		fmt.Println()
		fmt.Printf("  %s\n", o.dim.Render("kb-devkit run build"))
		fmt.Printf("  %s\n", o.dim.Render("kb-devkit run build lint test --affected"))
		fmt.Println()
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing devkit.yaml")
	rootCmd.AddCommand(initCmd)
}
