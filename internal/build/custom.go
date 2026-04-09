package build

import (
	"os"
	"os/exec"
	"time"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// CustomRunner executes a user-defined shell command for the build.
type CustomRunner struct {
	Command string
}

func (r *CustomRunner) Name() string { return "custom" }

func (r *CustomRunner) Build(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) BuildResult {
	start := time.Now()

	cmd := exec.Command("sh", "-c", r.Command)
	cmd.Dir = ws.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return BuildResult{
			OK:      false,
			Elapsed: time.Since(start),
			Hint:    "custom build command failed — see output above",
		}
	}

	return BuildResult{
		OK:      true,
		Elapsed: time.Since(start),
	}
}
