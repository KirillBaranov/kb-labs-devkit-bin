package cmd

import (
	"fmt"
	"os"

	"github.com/kb-labs/devkit/internal/runner"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <script>",
	Short: "Run a custom checker script",
	Long: `Executes a custom checker script as an escape hatch.

Contract:
  - $1 = package directory
  - exit 0 = pass
  - exit 1 = fail
  - stdout may contain JSON []Issue (optional)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		script := args[0]

		_, cfg, err := loadWorkspace()
		if err != nil {
			return err
		}
		_ = cfg

		// Run against cwd by default.
		cwd, _ := os.Getwd()
		issues, err := runner.RunCustom(script, cwd)
		if err != nil {
			return err
		}

		if jsonMode {
			_ = JSONOut(map[string]any{
				"ok":     len(issues) == 0,
				"issues": issues,
			})
			return errSilent
		}

		o := newOutput()
		if len(issues) == 0 {
			o.OK(fmt.Sprintf("Custom check %q passed", script))
			return nil
		}

		o.Err(fmt.Sprintf("Custom check %q reported %d issue(s):", script, len(issues)))
		for _, issue := range issues {
			fmt.Printf("  %s  %s\n", o.SeverityColor(string(issue.Severity)), issue.Message)
		}
		return fmt.Errorf("custom check failed")
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
