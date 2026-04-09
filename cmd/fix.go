package cmd

import (
	"fmt"

	"github.com/kb-labs/devkit/internal/checks"
	"github.com/spf13/cobra"
)

var (
	fixPackage string
	fixDryRun  bool
)

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Auto-fix issues where possible",
	Long: `Applies auto-fixes for issues flagged with autoFix: true.
Use --dry-run to preview what would be changed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, cfg, err := loadWorkspace()
		if err != nil {
			return err
		}

		pkgs := ws.Packages
		if fixPackage != "" {
			pkgs = ws.FilterByName([]string{fixPackage})
			if len(pkgs) == 0 {
				return fmt.Errorf("package %q not found", fixPackage)
			}
		}

		registry := checks.Default()
		results := checks.RunAll(ws, cfg, registry, nil)

		fixCount := 0
		var fixLog []string

		for _, r := range results {
			if r.Skipped {
				continue
			}

			cat, ok := cfg.Workspace.FindCategory(r.Package.Category)
			if !ok {
				continue
			}
			preset, err := cat.PresetConfig(cfg)
			if err != nil {
				continue
			}

			for _, rule := range registry.All() {
				fixer, ok := rule.(checks.Fixer)
				if !ok {
					continue
				}

				// Collect AutoFix issues for this rule.
				var fixable []checks.Issue
				for _, issue := range r.Issues {
					if issue.Check == rule.Name() && issue.AutoFix {
						fixable = append(fixable, issue)
					}
				}

				if len(fixable) == 0 {
					continue
				}

				if err := fixer.Apply(r.Package, fixable, fixDryRun); err != nil {
					return fmt.Errorf("fix %s in %s: %w", rule.Name(), r.Package.Name, err)
				}

				for _, issue := range fixable {
					fixLog = append(fixLog, fmt.Sprintf("%s: %s", r.Package.Name, issue.Message))
					fixCount++
				}
				_ = preset
			}
		}

		if jsonMode {
			_ = JSONOut(map[string]any{
				"ok":      true,
				"fixed":   fixCount,
				"dryRun":  fixDryRun,
				"actions": fixLog,
			})
			return errSilent
		}

		o := newOutput()
		if fixCount == 0 {
			o.OK("Nothing to fix")
			return nil
		}

		prefix := "Fixed"
		if fixDryRun {
			prefix = "Would fix"
		}
		o.OK(fmt.Sprintf("%s %d issue(s):", prefix, fixCount))
		for _, entry := range fixLog {
			fmt.Printf("  %s %s\n", o.bullet.Render("●"), entry)
		}

		return nil
	},
}

func init() {
	fixCmd.Flags().StringVar(&fixPackage, "package", "", "fix a single package by name")
	fixCmd.Flags().BoolVar(&fixDryRun, "dry-run", false, "preview fixes without applying")
	rootCmd.AddCommand(fixCmd)
}
