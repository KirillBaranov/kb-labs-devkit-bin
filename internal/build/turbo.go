package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// TurboRunner generates/updates turbo.json and delegates to `turbo run build`.
type TurboRunner struct{}

func (r *TurboRunner) Name() string { return "turbo" }

func (r *TurboRunner) Build(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) BuildResult {
	start := time.Now()

	// Ensure turbo is available.
	if _, err := exec.LookPath("turbo"); err != nil {
		return BuildResult{
			OK:      false,
			Elapsed: time.Since(start),
			Hint:    "turbo not found — install with: npm install -g turbo",
		}
	}

	// Generate/update turbo.json.
	if err := writeTurboJSON(ws.Root, ws.Packages); err != nil {
		return BuildResult{
			OK:      false,
			Elapsed: time.Since(start),
			Hint:    fmt.Sprintf("failed to write turbo.json: %v", err),
		}
	}

	args := []string{"run", "build"}
	if len(opts.Packages) > 0 {
		for _, pkg := range opts.Packages {
			args = append(args, "--filter="+pkg)
		}
	}
	if opts.Affected {
		args = append(args, "--affected")
	}

	cmd := exec.Command("turbo", args...)
	cmd.Dir = ws.Root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return BuildResult{
			OK:      false,
			Elapsed: time.Since(start),
			Hint:    "turbo build failed — see output above",
		}
	}

	return BuildResult{
		OK:      true,
		Elapsed: time.Since(start),
	}
}

type turboJSON struct {
	Schema  string                 `json:"$schema"`
	Tasks   map[string]turboTask   `json:"tasks"`
}

type turboTask struct {
	DependsOn []string `json:"dependsOn,omitempty"`
	Outputs   []string `json:"outputs,omitempty"`
	Cache     *bool    `json:"cache,omitempty"`
}

func writeTurboJSON(root string, pkgs []workspace.Package) error {
	cacheTrue := true
	tj := turboJSON{
		Schema: "https://turbo.build/schema.json",
		Tasks: map[string]turboTask{
			"build": {
				DependsOn: []string{"^build"},
				Outputs:   []string{"dist/**"},
				Cache:     &cacheTrue,
			},
			"dev": {
				DependsOn: []string{"^build"},
				Cache:     boolPtr(false),
			},
			"test": {
				DependsOn: []string{"build"},
				Outputs:   []string{},
			},
			"lint": {
				Outputs: []string{},
			},
			"type-check": {
				DependsOn: []string{"^build"},
				Outputs:   []string{},
			},
		},
	}

	data, err := json.MarshalIndent(tj, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(root, "turbo.json"), append(data, '\n'), 0o644)
}

func boolPtr(b bool) *bool { return &b }
