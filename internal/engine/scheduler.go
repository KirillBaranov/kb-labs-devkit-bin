package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/kb-labs/devkit/internal/cache"
	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// RunOptions controls the scheduler behaviour.
type RunOptions struct {
	Tasks       []string // task names to run (in order)
	Packages    []workspace.Package
	NoCache     bool
	Concurrency int
	WSRoot      string
	CacheRoot   string // path to .kb/devkit/
}

// RunResult is the aggregate result of a scheduler run.
type RunResult struct {
	OK      bool
	Results []TaskResult
}

func (r RunResult) Summary() RunSummary {
	s := RunSummary{Total: len(r.Results)}
	for _, res := range r.Results {
		switch {
		case res.Cached:
			s.Cached++
		case res.OK:
			s.Passed++
		default:
			s.Failed++
		}
	}
	return s
}

// RunSummary is the JSON summary envelope.
type RunSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
	Cached int `json:"cached"`
}

// Run builds the (package × task) DAG and executes it in topological order.
// Packages in the same layer with no inter-task dependencies run in parallel.
func Run(ws *workspace.Workspace, cfg *config.DevkitConfig, opts RunOptions) (RunResult, error) {
	if len(opts.Tasks) == 0 {
		return RunResult{}, fmt.Errorf("no tasks specified")
	}

	pkgs := opts.Packages
	if len(pkgs) == 0 {
		pkgs = ws.Packages
	}

	// Resolve task definitions.
	taskDefs := ResolveTaskDefs(cfg, opts.Tasks)

	// Validate all tasks have commands.
	for _, name := range opts.Tasks {
		def := taskDefs[name]
		if def.Command == "" {
			return RunResult{}, fmt.Errorf("unknown task %q — add it to devkit.yaml tasks: section", name)
		}
	}

	// Init cache.
	cacheRoot := opts.CacheRoot
	if cacheRoot == "" {
		cacheRoot = ".kb/devkit"
	}
	objects, err := cache.NewLocalStore(cacheRoot)
	if err != nil {
		return RunResult{}, fmt.Errorf("cache init: %w", err)
	}
	manifests := cache.NewManifestStore(cacheRoot)
	executor := NewExecutor(objects, manifests, opts.WSRoot)

	// Build DAG of (pkg, task) nodes.
	nodes, err := buildDAG(pkgs, opts.Tasks, taskDefs, ws)
	if err != nil {
		return RunResult{}, err
	}

	// Concurrency.
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = runtime.NumCPU()
		if concurrency > 1 {
			concurrency--
		}
	}

	// Execute layer by layer (Kahn's algorithm).
	inDegree := make(map[nodeKey]int, len(nodes))
	revDeps := make(map[nodeKey][]nodeKey)

	for k, n := range nodes {
		if _, ok := inDegree[k]; !ok {
			inDegree[k] = 0
		}
		for _, dep := range n.deps {
			inDegree[k]++
			revDeps[dep] = append(revDeps[dep], k)
		}
	}

	queue := make([]nodeKey, 0)
	for k, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, k)
		}
	}

	var allResults []TaskResult
	failed := false

	for len(queue) > 0 {
		layer := queue
		queue = nil

		layerResults := runLayer(layer, nodes, executor, taskDefs, opts.NoCache, concurrency)
		allResults = append(allResults, layerResults...)

		for _, r := range layerResults {
			if !r.OK && !r.Cached {
				failed = true
			}
		}

		if failed {
			break
		}

		for _, k := range layer {
			for _, dependent := range revDeps[k] {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					queue = append(queue, dependent)
				}
			}
		}
	}

	ok := true
	for _, r := range allResults {
		if !r.OK && !r.Cached {
			ok = false
			break
		}
	}

	return RunResult{OK: ok, Results: allResults}, nil
}

// ─── DAG construction ─────────────────────────────────────────────────────────

type nodeKey struct {
	pkg  string // package name
	task string
}

type dagNode struct {
	pkg  workspace.Package
	task string
	deps []nodeKey
}

func buildDAG(
	pkgs []workspace.Package,
	taskNames []string,
	taskDefs map[string]TaskDef,
	ws *workspace.Workspace,
) (map[nodeKey]dagNode, error) {
	// Build package name → Package map.
	pkgByName := make(map[string]workspace.Package, len(pkgs))
	for _, p := range pkgs {
		pkgByName[p.Name] = p
	}

	// Build dep map: pkg name → list of workspace dep names.
	pkgDeps := buildPkgDepMap(pkgs, pkgByName)

	nodes := make(map[nodeKey]dagNode)

	// Create a node for every (pkg, task) pair.
	for _, p := range pkgs {
		for _, taskName := range taskNames {
			k := nodeKey{p.Name, taskName}
			nodes[k] = dagNode{pkg: p, task: taskName}
		}
	}

	// Resolve deps for each node.
	for k, n := range nodes {
		def := taskDefs[k.task]
		var deps []nodeKey

		for _, dep := range def.Deps {
			if strings.HasPrefix(dep, "^") {
				// ^build = run 'build' for every workspace dependency first.
				depTask := strings.TrimPrefix(dep, "^")
				for _, depPkgName := range pkgDeps[k.pkg] {
					if _, exists := pkgByName[depPkgName]; exists {
						depKey := nodeKey{depPkgName, depTask}
						if _, hasNode := nodes[depKey]; hasNode {
							deps = append(deps, depKey)
						}
					}
				}
			} else {
				// "build" = run 'build' for this same package first.
				depKey := nodeKey{k.pkg, dep}
				if _, hasNode := nodes[depKey]; hasNode {
					deps = append(deps, depKey)
				}
			}
		}

		n.deps = deps
		nodes[k] = n
	}

	return nodes, nil
}

// buildPkgDepMap reads package.json deps for each package.
// Returns: pkg name → list of workspace dep names.
func buildPkgDepMap(pkgs []workspace.Package, pkgByName map[string]workspace.Package) map[string][]string {
	result := make(map[string][]string, len(pkgs))
	for _, p := range pkgs {
		result[p.Name] = readWorkspaceDeps(p.Dir, pkgByName)
	}
	return result
}

// readWorkspaceDeps returns workspace-internal dependency names for a package.
func readWorkspaceDeps(dir string, pkgByName map[string]workspace.Package) []string {
	data, err := os.ReadFile(dir + "/package.json")
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	var result []string
	for name := range pkg.Dependencies {
		if _, ok := pkgByName[name]; ok {
			result = append(result, name)
		}
	}
	return result
}

// ─── Layer execution ──────────────────────────────────────────────────────────

func runLayer(
	layer []nodeKey,
	nodes map[nodeKey]dagNode,
	executor *Executor,
	taskDefs map[string]TaskDef,
	noCache bool,
	concurrency int,
) []TaskResult {
	results := make([]TaskResult, len(layer))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, k := range layer {
		i, k := i, k
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			n := nodes[k]
			def := taskDefs[k.task]
			results[i] = executor.Run(n.pkg, def, noCache)
		}()
	}

	wg.Wait()
	return results
}

// ─── Affected packages via git ────────────────────────────────────────────────

// AffectedPackages returns packages with files changed since last commit.
// V1: directly changed packages only (no downstream BFS).
func AffectedPackages(ws *workspace.Workspace) ([]workspace.Package, error) {
	// git diff --name-only HEAD (staged + unstaged changes).
	out, err := exec.Command("git", "diff", "--name-only", "HEAD").Output()
	if err != nil {
		// Fallback: staged only (unborn branch / initial commit).
		out, err = exec.Command("git", "diff", "--name-only", "--cached").Output()
		if err != nil {
			return nil, fmt.Errorf("git diff: %w", err)
		}
	}

	changedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")

	seen := make(map[string]bool)
	var affected []workspace.Package

	for _, relFile := range changedFiles {
		if relFile == "" {
			continue
		}
		absFile := ws.Root + "/" + relFile
		pkg, ok := ws.PackageByPath(absFile)
		if ok && !seen[pkg.Name] {
			seen[pkg.Name] = true
			affected = append(affected, *pkg)
		}
	}

	return affected, nil
}
