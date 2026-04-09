package cmd

import (
	"fmt"

	"github.com/kb-labs/devkit/internal/build"
	"github.com/spf13/cobra"
)

var (
	buildRunner   string
	buildAffected bool
	buildPackages []string
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build packages in dependency order",
	Long: `Builds all packages in the workspace in topological dependency order.

Runners:
  native  — builds dependency graph, runs layers in parallel (default)
  turbo   — generates turbo.json and delegates to turbo run build
  custom  — executes build.command from devkit.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, cfg, err := loadWorkspace()
		if err != nil {
			return err
		}

		opts := build.BuildOptions{
			Packages:    buildPackages,
			Affected:    buildAffected,
			Cache:       cfg.Build.Cache,
			Concurrency: cfg.Build.Concurrency,
			Runner:      buildRunner,
		}

		result, err := build.Dispatch(ws, cfg, opts)
		if err != nil {
			return err
		}

		if jsonMode {
			summary := result.Summary()
			_ = JSONOut(map[string]any{
				"ok":       result.OK,
				"packages": result.Packages,
				"summary":  summary,
				"elapsed":  result.Elapsed.String(),
				"hint":     result.Hint,
			})
			if !result.OK {
				return errSilent
			}
			return errSilent
		}

		// Human output.
		o := newOutput()
		fmt.Println()
		for _, pkg := range result.Packages {
			icon := o.StatusIcon("healthy")
			label := ""
			switch {
			case pkg.Skipped:
				icon = o.dim.Render("-")
				label = o.dim.Render("up-to-date")
			case pkg.OK:
				label = o.dim.Render(pkg.Elapsed.Round(1000000).String())
			default:
				icon = o.StatusIcon("error")
				label = o.errStyle.Render("FAILED")
			}
			fmt.Printf("  %s %-40s  %s\n", icon, pkg.Name, label)
		}

		s := result.Summary()
		fmt.Printf("\n  %d passed, %d failed, %d skipped — %s\n\n",
			s.Passed, s.Failed, s.Skipped, result.Elapsed.Round(1000000))

		if !result.OK {
			return fmt.Errorf("build failed")
		}
		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&buildRunner, "runner", "", "build runner: native, turbo, custom (overrides devkit.yaml)")
	buildCmd.Flags().BoolVar(&buildAffected, "affected", false, "build only packages changed since last build")
	buildCmd.Flags().StringSliceVar(&buildPackages, "packages", nil, "build specific packages (comma-separated names)")
	rootCmd.AddCommand(buildCmd)
}
