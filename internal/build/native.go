package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// NativeRunner builds packages in dependency order using topological sort.
// Packages in the same layer are built in parallel.
// Invokes tsup/go directly — bypasses pnpm run overhead (~300-500ms per package).
type NativeRunner struct{}

func (r *NativeRunner) Name() string { return "native" }

func (r *NativeRunner) Build(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) BuildResult {
	start := time.Now()

	pkgs := ws.Packages
	if len(opts.Packages) > 0 {
		pkgs = ws.FilterByName(opts.Packages)
	}

	layers := topoSort(pkgs)

	// Concurrency: use config, flag override, or auto-detect from CPU count.
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = cfg.Build.Concurrency
	}
	if concurrency <= 0 {
		// Default: leave one core free for the OS / other processes.
		concurrency = runtime.NumCPU()
		if concurrency > 1 {
			concurrency--
		}
	}

	// Resolve tsup binary once for the whole build.
	// Look in workspace root first, then fall back to PATH.
	tsupBin := findBin(ws.Root, "tsup")

	var allResults []PackageBuildResult

	for layerIdx, layer := range layers {
		layerResults := buildLayer(layer, opts, concurrency, tsupBin, ws.Root, layerIdx)
		allResults = append(allResults, layerResults...)

		// Stop on first layer with failures.
		for _, r := range layerResults {
			if !r.OK && !r.Skipped {
				return BuildResult{
					OK:       false,
					Packages: allResults,
					Elapsed:  time.Since(start),
					Hint:     fmt.Sprintf("build failed at %s — fix errors before continuing", r.Name),
				}
			}
		}
	}

	ok := true
	for _, r := range allResults {
		if !r.OK && !r.Skipped {
			ok = false
			break
		}
	}

	return BuildResult{
		OK:       ok,
		Packages: allResults,
		Elapsed:  time.Since(start),
		Layers:   len(layers),
	}
}

func buildLayer(pkgs []workspace.Package, opts BuildOptions, concurrency int, tsupBin, wsRoot string, layerIdx int) []PackageBuildResult {
	results := make([]PackageBuildResult, len(pkgs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, pkg := range pkgs {
		i, pkg := i, pkg
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = buildPackage(pkg, opts, tsupBin, wsRoot, layerIdx)
		}()
	}

	wg.Wait()
	return results
}

func buildPackage(pkg workspace.Package, opts BuildOptions, tsupBin, wsRoot string, layerIdx int) PackageBuildResult {
	start := time.Now()

	// Cache check: skip if dist/ is newer than all src/ files.
	if opts.Cache {
		if upToDate, reason := isUpToDate(pkg.Dir); upToDate {
			return PackageBuildResult{
				Name:    pkg.Name,
				OK:      true,
				Skipped: true,
				Reason:  reason,
				Layer:   layerIdx,
				Elapsed: time.Since(start),
			}
		}
	}

	var cmd *exec.Cmd

	switch pkg.Language {
	case "go":
		cmd = buildGo(pkg)
	default:
		cmd = buildTS(pkg, tsupBin, wsRoot)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return PackageBuildResult{
			Name:    pkg.Name,
			OK:      false,
			Layer:   layerIdx,
			Elapsed: time.Since(start),
			Error:   fmt.Sprintf("%v\n%s", err, string(out)),
		}
	}

	return PackageBuildResult{
		Name:    pkg.Name,
		OK:      true,
		Layer:   layerIdx,
		Elapsed: time.Since(start),
	}
}

// buildTS runs tsup directly — skips pnpm run overhead (~300-500ms per package).
// Falls back to `pnpm run build` if no tsup config found.
func buildTS(pkg workspace.Package, tsupBin, wsRoot string) *exec.Cmd {
	// Check if tsup config exists.
	tsupConfig := findTsupConfig(pkg.Dir)

	if tsupBin != "" && tsupConfig != "" {
		// Direct tsup invocation — fastest path.
		args := []string{"--config", tsupConfig}
		cmd := exec.Command(tsupBin, args...)
		cmd.Dir = pkg.Dir
		// Inherit NODE_PATH so tsup can resolve @kb-labs/* from workspace node_modules.
		cmd.Env = append(os.Environ(),
			"NODE_PATH="+filepath.Join(wsRoot, "node_modules"),
		)
		return cmd
	}

	// Fallback: pnpm run build (slower but always works).
	cmd := exec.Command("pnpm", "run", "build")
	cmd.Dir = pkg.Dir
	return cmd
}

// buildGo runs `go build ./...` for Go packages.
func buildGo(pkg workspace.Package) *exec.Cmd {
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = pkg.Dir
	return cmd
}

// findBin finds a binary in <wsRoot>/node_modules/.bin/ or falls back to PATH.
func findBin(wsRoot, name string) string {
	candidate := filepath.Join(wsRoot, "node_modules", ".bin", name)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// Try PATH.
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// findTsupConfig returns the path to tsup config file in pkg dir, or "".
func findTsupConfig(dir string) string {
	candidates := []string{
		"tsup.config.ts",
		"tsup.config.js",
		"tsup.config.mjs",
		"tsup.config.cjs",
	}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// isUpToDate returns (true, reason) if dist/ is newer than all src/** files.
// Uses the newest file in dist/ as the reference point (not folder mtime).
func isUpToDate(dir string) (bool, string) {
	distDir := filepath.Join(dir, "dist")
	srcDir := filepath.Join(dir, "src")

	if _, err := os.Stat(distDir); err != nil {
		return false, "no dist/"
	}
	if _, err := os.Stat(srcDir); err != nil {
		return false, "no src/"
	}

	// Find newest file in dist/.
	var distNewest time.Time
	_ = filepath.WalkDir(distDir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(distNewest) {
			distNewest = info.ModTime()
		}
		return nil
	})

	if distNewest.IsZero() {
		return false, "dist/ is empty"
	}

	// Check if any src/ file is newer than distNewest.
	outdated := false
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(distNewest) {
			outdated = true
			return filepath.SkipAll
		}
		return nil
	})

	if outdated {
		return false, ""
	}
	return true, "src unchanged"
}

// topoSort performs a topological sort (Kahn's algorithm) on packages.
// Returns packages grouped into parallel build layers.
func topoSort(pkgs []workspace.Package) [][]workspace.Package {
	// Build name → index map.
	nameToIdx := make(map[string]int, len(pkgs))
	for i, p := range pkgs {
		nameToIdx[p.Name] = i
	}

	// Read all package.json deps once.
	allDeps := make([][]int, len(pkgs))
	for i, pkg := range pkgs {
		allDeps[i] = readInternalDepNames(pkg.Dir, nameToIdx)
	}

	// In-degree and reverse adjacency from the pre-read deps.
	inDegree := make([]int, len(pkgs))
	revDeps := make([][]int, len(pkgs))
	for i, deps := range allDeps {
		for _, dep := range deps {
			inDegree[i]++
			revDeps[dep] = append(revDeps[dep], i)
		}
	}

	// Kahn's algorithm.
	var queue []int
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	var layers [][]workspace.Package
	for len(queue) > 0 {
		layer := queue
		queue = nil

		var pkgLayer []workspace.Package
		for _, idx := range layer {
			pkgLayer = append(pkgLayer, pkgs[idx])
			for _, dependent := range revDeps[idx] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					queue = append(queue, dependent)
				}
			}
		}
		layers = append(layers, pkgLayer)
	}

	return layers
}

func readInternalDepNames(dir string, nameToIdx map[string]int) []int {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var result []int
	for name := range pkg.Dependencies {
		if idx, ok := nameToIdx[name]; ok {
			result = append(result, idx)
		}
	}
	return result
}
