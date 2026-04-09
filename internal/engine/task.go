// Package engine implements the task execution engine.
// A task is any command with declared inputs, outputs, and dependencies.
// The engine caches results by input hash — same inputs → restore from cache.
package engine

import (
	"time"

	"github.com/kb-labs/devkit/internal/config"
)

// TaskDef is the resolved definition of one named task.
type TaskDef struct {
	Name    string
	Command string
	Inputs  []string // glob patterns relative to package dir
	Outputs []string // glob patterns relative to package dir
	// Deps: "^build" = run 'build' for all workspace deps first
	//       "build"  = run 'build' for this package first
	Deps  []string
	Cache bool // default true; false = always run (e.g. deploy)
}

// TaskResult is the outcome of running one (package, task) pair.
type TaskResult struct {
	Package  string
	Task     string
	OK       bool
	Cached   bool
	Elapsed  time.Duration
	ExitCode int
	Stdout   string
	Stderr   string
	Error    string
}

// ResolveTaskDefs returns the effective task definitions for a given config.
// Config tasks override built-in defaults; missing tasks fall back to defaults.
func ResolveTaskDefs(cfg *config.DevkitConfig, names []string) map[string]TaskDef {
	defaults := defaultTaskDefs()
	result := make(map[string]TaskDef, len(names))

	for _, name := range names {
		def, ok := defaults[name]

		// Config override.
		if cfg != nil {
			if ct, has := cfg.Tasks[name]; has {
				cacheVal := true
				if ct.Cache != nil {
					cacheVal = *ct.Cache
				}
				def = TaskDef{
					Name:    name,
					Command: ct.Command,
					Inputs:  ct.Inputs,
					Outputs: ct.Outputs,
					Deps:    ct.Deps,
					Cache:   cacheVal,
				}
				ok = true
			}
		}

		if !ok {
			// Unknown task — create minimal def so the engine can report the error.
			def = TaskDef{Name: name, Cache: true}
		}

		result[name] = def
	}

	return result
}

// defaultTaskDefs returns built-in task definitions that work for any
// node-lib / node-app preset without configuration.
func defaultTaskDefs() map[string]TaskDef {
	return map[string]TaskDef{
		"build": {
			Name:    "build",
			Command: "tsup",
			Inputs:  []string{"src/**", "tsup.config.ts", "tsup.config.js", "tsconfig*.json"},
			Outputs: []string{"dist/**"},
			Deps:    []string{"^build"},
			Cache:   true,
		},
		"lint": {
			Name:    "lint",
			Command: "eslint src/",
			Inputs:  []string{"src/**", "eslint.config.*", ".eslintrc*", ".eslintignore"},
			Outputs: []string{},
			Deps:    []string{},
			Cache:   true,
		},
		"type-check": {
			Name:    "type-check",
			Command: "tsc --noEmit",
			Inputs:  []string{"src/**", "tsconfig.json", "tsconfig*.json"},
			Outputs: []string{},
			Deps:    []string{"^build"},
			Cache:   true,
		},
		"test": {
			Name:    "test",
			Command: "vitest run",
			Inputs:  []string{"src/**", "test/**", "__tests__/**", "vitest.config.*"},
			Outputs: []string{"coverage/**"},
			Deps:    []string{"build"},
			Cache:   true,
		},
	}
}
