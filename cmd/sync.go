package cmd

import (
	"fmt"

	devsync "github.com/kb-labs/devkit/internal/sync"
	"github.com/spf13/cobra"
)

var (
	syncCheck  bool
	syncDryRun bool
	syncSource string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync configs and assets from declared sources to submodules",
	Long: `Applies sync targets defined in devkit.yaml to every submodule.

Sources:
  npm   — reads from node_modules/<package>/
  local — reads from a local directory

Use --check to report drift without writing.
Use --dry-run to preview changes without writing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ws, cfg, err := loadWorkspace()
		if err != nil {
			return err
		}

		if len(cfg.Sync.Targets) == 0 {
			return fmt.Errorf("no sync targets defined in devkit.yaml")
		}

		syncer := devsync.New(ws.Root, cfg, ws)

		opts := devsync.Options{
			Check:  syncCheck,
			DryRun: syncDryRun,
			Source: syncSource,
		}

		result, err := syncer.Run(opts)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		if jsonMode {
			_ = JSONOut(map[string]any{
				"ok":      len(result.Drifted) == 0 || !syncCheck,
				"created": result.Created,
				"updated": result.Updated,
				"skipped": result.Skipped,
				"drifted": result.Drifted,
			})
			return errSilent
		}

		o := newOutput()

		if syncCheck {
			if len(result.Drifted) == 0 {
				o.OK("No drift detected — all files match source")
			} else {
				o.Warn(fmt.Sprintf("%d file(s) drifted from source:", len(result.Drifted)))
				for _, f := range result.Drifted {
					fmt.Printf("  %s %s\n", o.warning.Render("~"), f)
				}
			}
			return nil
		}

		if syncDryRun {
			fmt.Printf("\n%s\n", o.label.Render("Dry run — no files written"))
		}

		for _, f := range result.Created {
			fmt.Printf("  %s %s\n", o.healthy.Render("+"), f)
		}
		for _, f := range result.Updated {
			fmt.Printf("  %s %s\n", o.info.Render("~"), f)
		}
		for _, f := range result.Skipped {
			fmt.Printf("  %s %s\n", o.dim.Render("-"), f)
		}

		if !syncDryRun {
			o.OK(fmt.Sprintf("Sync complete: %d created, %d updated, %d skipped",
				len(result.Created), len(result.Updated), len(result.Skipped)))
		}

		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncCheck, "check", false, "report drift only, do not write")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "preview changes without writing")
	syncCmd.Flags().StringVar(&syncSource, "source", "", "limit sync to a specific source key")
	rootCmd.AddCommand(syncCmd)
}
