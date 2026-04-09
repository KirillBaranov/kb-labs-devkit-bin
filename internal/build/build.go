// Package build implements pluggable build runners.
package build

import (
	"fmt"
	"time"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// PackageBuildResult is the outcome for one package.
type PackageBuildResult struct {
	Name    string        `json:"name"`
	OK      bool          `json:"ok"`
	Skipped bool          `json:"skipped"` // up-to-date (mtime cache hit)
	Elapsed time.Duration `json:"elapsed"`
	Error   string        `json:"error,omitempty"`
}

// BuildResult is the aggregate outcome for a build run.
type BuildResult struct {
	OK       bool                 `json:"ok"`
	Packages []PackageBuildResult `json:"packages"`
	Elapsed  time.Duration        `json:"elapsed"`
	Hint     string               `json:"hint,omitempty"`
}

// BuildSummary is the JSON summary envelope.
type BuildSummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

func (r BuildResult) Summary() BuildSummary {
	s := BuildSummary{Total: len(r.Packages)}
	for _, p := range r.Packages {
		switch {
		case p.Skipped:
			s.Skipped++
		case p.OK:
			s.Passed++
		default:
			s.Failed++
		}
	}
	return s
}

// BuildOptions controls what gets built.
type BuildOptions struct {
	Packages    []string // nil = all
	Affected    bool     // only packages changed since last build
	Cache       bool
	Concurrency int
	Runner      string // override config runner
}

// Runner is the interface all build runners must implement.
type Runner interface {
	Name() string
	Build(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) BuildResult
}

// Dispatch selects and runs the appropriate build runner.
func Dispatch(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) (BuildResult, error) {
	runnerName := opts.Runner
	if runnerName == "" {
		runnerName = cfg.Build.Runner
	}
	if runnerName == "" {
		runnerName = "native"
	}

	var runner Runner
	switch runnerName {
	case "native":
		runner = &NativeRunner{}
	case "turbo":
		runner = &TurboRunner{}
	case "custom":
		if cfg.Build.Command == "" {
			return BuildResult{}, fmt.Errorf("build.runner=custom requires build.command to be set")
		}
		runner = &CustomRunner{Command: cfg.Build.Command}
	default:
		return BuildResult{}, fmt.Errorf("unknown build runner %q (want: native, turbo, custom)", runnerName)
	}

	result := runner.Build(ws, cfg, opts)
	return result, nil
}
