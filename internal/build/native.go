package build

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// NativeRunner builds packages in dependency order using topological sort.
// Packages in the same layer are built in parallel.
type NativeRunner struct{}

func (r *NativeRunner) Name() string { return "native" }

func (r *NativeRunner) Build(ws *workspace.Workspace, cfg *config.DevkitConfig, opts BuildOptions) BuildResult {
	start := time.Now()

	pkgs := ws.Packages
	if len(opts.Packages) > 0 {
		pkgs = ws.FilterByName(opts.Packages)
	}

	layers := topoSort(pkgs)

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = cfg.Build.Concurrency
	}
	if concurrency <= 0 {
		concurrency = 8
	}

	var allResults []PackageBuildResult

	for _, layer := range layers {
		layerResults := buildLayer(layer, opts, concurrency)
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
	}
}

func buildLayer(pkgs []workspace.Package, opts BuildOptions, concurrency int) []PackageBuildResult {
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
			results[i] = buildPackage(pkg, opts)
		}()
	}

	wg.Wait()
	return results
}

func buildPackage(pkg workspace.Package, opts BuildOptions) PackageBuildResult {
	start := time.Now()

	// Cache check: skip if dist/ is newer than all src/ files.
	if opts.Cache {
		if isUpToDate(pkg.Dir) {
			return PackageBuildResult{
				Name:    pkg.Name,
				OK:      true,
				Skipped: true,
				Elapsed: time.Since(start),
			}
		}
	}

	// Run the build script.
	cmd := exec.Command("pnpm", "run", "build")
	cmd.Dir = pkg.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return PackageBuildResult{
			Name:    pkg.Name,
			OK:      false,
			Elapsed: time.Since(start),
			Error:   fmt.Sprintf("%v\n%s", err, string(out)),
		}
	}

	return PackageBuildResult{
		Name:    pkg.Name,
		OK:      true,
		Elapsed: time.Since(start),
	}
}

// isUpToDate returns true if dist/ is newer than all src/** files.
func isUpToDate(dir string) bool {
	distStat, err := os.Stat(filepath.Join(dir, "dist"))
	if err != nil {
		return false
	}
	distMtime := distStat.ModTime()

	srcDir := filepath.Join(dir, "src")
	if _, err := os.Stat(srcDir); err != nil {
		return false
	}

	outdated := false
	_ = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(distMtime) {
			outdated = true
			return filepath.SkipAll
		}
		return nil
	})

	return !outdated
}

// topoSort performs a topological sort (Kahn's algorithm) on packages.
// Returns packages grouped into parallel build layers.
func topoSort(pkgs []workspace.Package) [][]workspace.Package {
	// Build name → index map.
	nameToIdx := make(map[string]int, len(pkgs))
	for i, p := range pkgs {
		nameToIdx[p.Name] = i
	}

	// Build adjacency list and in-degree count.
	inDegree := make([]int, len(pkgs))
	deps := make([][]int, len(pkgs)) // deps[i] = list of package indices that i depends on

	for i, pkg := range pkgs {
		internal := readInternalDepNames(pkg.Dir, nameToIdx)
		for _, dep := range internal {
			deps[i] = append(deps[i], dep)
			inDegree[i]++
		}
	}

	// Reverse: who depends on me?
	revDeps := make([][]int, len(pkgs))
	for i, pkg := range pkgs {
		internal := readInternalDepNames(pkg.Dir, nameToIdx)
		_ = pkg
		for _, dep := range internal {
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
