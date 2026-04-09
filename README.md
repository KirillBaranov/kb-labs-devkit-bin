# kb-devkit

Go binary for workspace orchestration: quality checks, config sync, and task execution with content-addressable caching.

## Build

```bash
make build
```

## Commands

### `run` — task execution engine

Runs named tasks across all packages in dependency order. Results are cached by input hash — identical inputs skip execution and restore outputs in milliseconds.

```bash
kb-devkit run build
kb-devkit run build lint
kb-devkit run build lint test --affected
kb-devkit run build --packages @kb-labs/core-types,@kb-labs/core-runtime
kb-devkit run build --no-cache
kb-devkit run build --live          # stream stdout/stderr in real time
kb-devkit run build --json          # machine-readable output
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--affected` | Run only changed packages + all downstream dependents |
| `--packages` | Comma-separated list of package names to run |
| `--no-cache` | Bypass cache lookup (still stores result for next run) |
| `--live` | Stream output while running (forces `concurrency=1`) |
| `--concurrency N` | Max parallel tasks (default: `NumCPU-1`) |

**How caching works:**

1. Hash all input files matching `inputs:` glob patterns
2. Cache hit? → restore output files + return in ~1ms
3. Cache miss → run command → store outputs → write manifest

Cache lives in `.kb/devkit/`. Objects are content-addressable (SHA256), so the same file in two packages is stored once.

**`--affected` detection:**

Controlled by `affected.strategy` in `devkit.yaml`:

| Strategy | Behaviour |
|----------|-----------|
| `git` | Single `git diff --name-only HEAD` from workspace root (default) |
| `submodules` | Walks `.gitmodules`, runs `git diff` inside each submodule |
| `command` | Runs `affected.command`, reads one file path per line from stdout |

After finding directly changed packages, BFS expands through the reverse dependency graph to include all downstream dependents.

---

### `check` — quality checks

```bash
kb-devkit check                           # check all packages
kb-devkit check --package @kb-labs/core  # check single package
kb-devkit check --json                    # machine-readable output
```

### `stats` — workspace health

```bash
kb-devkit stats                           # health score A–F, issues breakdown
kb-devkit stats --json
```

### `status` — package table

```bash
kb-devkit status                          # table: name, category, preset, issues
kb-devkit status --json
```

### `sync` — config sync

```bash
kb-devkit sync --check                    # report drift without applying
kb-devkit sync --dry-run                  # preview changes
kb-devkit sync                            # apply sync
```

### `build` — native build runner

```bash
kb-devkit build                           # build all packages in topo order
kb-devkit build --affected                # only changed packages
kb-devkit build --runner turbo            # via turbo
```

### `watch` — file watcher

```bash
kb-devkit watch --json                    # stream violations as JSONL on save
```

### `gate` — pre-commit gate

```bash
kb-devkit gate                            # check staged files only; exits 1 on violations
```

### `doctor` — environment diagnostics

```bash
kb-devkit doctor --json
```

---

## Configuration (`devkit.yaml`)

```yaml
version: 1

workspace:
  packageManager: pnpm
  maxDepth: 3
  categories:
    ts-lib:
      match: ["platform/*/packages/**"]
      language: typescript
      preset: node-lib

# ── Task execution ───────────────────────────────────────────────────────────

tasks:
  build:
    command: tsup
    inputs:
      - "src/**"
      - "tsup.config.ts"
      - "tsconfig*.json"
    outputs:
      - "dist/**"
    deps:
      - "^build"      # run 'build' for all workspace deps first

  lint:
    command: eslint src/
    inputs:
      - "src/**"
      - "eslint.config.*"
    outputs: []
    deps: []

  type-check:
    command: tsc --noEmit
    inputs:
      - "src/**"
      - "tsconfig*.json"
    outputs: []
    deps:
      - "^build"      # needs deps' dist/**/*.d.ts

  test:
    command: vitest run --passWithNoTests
    inputs:
      - "src/**"
      - "test/**"
      - "vitest.config.*"
    outputs:
      - "coverage/**"
    deps:
      - "build"       # tests run after own build

  deploy:
    command: ./scripts/deploy.sh
    inputs:
      - "dist/**"
    outputs: []
    cache: false      # always runs, never cached

# ── Affected detection ───────────────────────────────────────────────────────

affected:
  # strategy: git (default) | submodules | command
  strategy: submodules
  # command: ./scripts/changed-files.sh   # used when strategy: command

# ── Presets, sync, build runner … (see devkit.yaml in workspace root) ────────
```

### Dep syntax

| Value | Meaning |
|-------|---------|
| `^build` | Run `build` for every workspace dependency first |
| `build` | Run `build` for this same package first |

### `cache: false`

Set on tasks that must always run (e.g. deploy, publish). The result is not stored.

---

## Cache layout

```
.kb/devkit/
  objects/
    ab/
      cdef1234...    ← raw file content, keyed by SHA256
  tasks/
    @kb-labs__core-types/
      build/
        abc123.json  ← manifest: input hash → output file refs + stdout
  tmp/
    staging-*        ← atomic write staging area
```

Same file content in two packages → one object. Rename-only change → new manifest, same objects.

---

## Architecture

```
cmd/
  run.go          ← kb-devkit run (task engine entry)
  check.go        ← kb-devkit check
  build.go        ← kb-devkit build (native runner)
  …

internal/
  cache/
    hash.go       ← InputHasher: globs → SHA256
    store.go      ← ObjectStore (LocalStore + interface for S3/R2)
    manifest.go   ← Manifest: inputHash → output refs + stdout
  engine/
    task.go       ← TaskDef, TaskResult, ResolveTaskDefs
    executor.go   ← hash → lookup → run → store
    scheduler.go  ← DAG builder (Kahn's), layer parallelism, AffectedPackages
  config/
    config.go     ← DevkitConfig, TaskConfig, AffectedConfig, …
    yaml.go       ← YAML parsing + mapping
  workspace/
    workspace.go  ← Package, Workspace, PackageByPath
    discover.go   ← glob-based package discovery
  checks/         ← individual quality check implementations
  build/          ← NativeRunner, TurboRunner
```
