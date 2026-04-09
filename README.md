# kb-devkit

Go binary for workspace orchestration: quality checks, config sync, and task execution with content-addressable caching.

## Build

```bash
make build
```

## Quick start

```bash
# Create starter config
kb-devkit init

# Build all packages (cached)
kb-devkit run build

# Second run — everything cached, <1s
kb-devkit run build

# Only changed packages + downstream dependents
kb-devkit run build lint test --affected
```

---

## Commands

### `init` — create devkit.yaml

```bash
kb-devkit init           # creates devkit.yaml with all sections commented as examples
kb-devkit init --force   # overwrite existing
```

Generates a fully commented `devkit.yaml`. Uncomment the tasks you need and adjust.

---

### `run` — task execution engine

Runs named tasks across all packages in dependency order with content-addressable caching.
Tasks are defined in `devkit.yaml`. Each task can have multiple **variants** — one per package category.

```bash
kb-devkit run build
kb-devkit run build lint
kb-devkit run build lint test --affected
kb-devkit run build --packages @kb-labs/core-types,@kb-labs/core-runtime
kb-devkit run build --no-cache
kb-devkit run build --live          # stream stdout/stderr in real time
kb-devkit run build --json
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--affected` | Run only changed packages + all downstream dependents |
| `--packages` | Comma-separated list of package names |
| `--no-cache` | Bypass cache lookup (still stores result) |
| `--live` | Stream output in real time (forces `concurrency=1`) |
| `--concurrency N` | Max parallel tasks (default: `run.concurrency` in yaml, then `NumCPU-1`) |

**How caching works:**

1. Hash all input files matching `inputs:` glob patterns → SHA256 key
2. Cache hit → restore output files in ~1ms, mark `cached`
3. Cache miss → run command → store outputs → write manifest

Cache lives in `.kb/devkit/`. Objects are content-addressable — the same file content in two packages is stored once.

Each task has its own independent cache keyed by `(taskName, package, inputHash)`. Running `build` does not populate the `lint` cache — they track different inputs and are stored separately. The only connection between tasks is `deps:` — if `lint` declares `deps: ["build"]`, build runs first (from cache if available), then lint runs fresh.

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
kb-devkit check --json
```

Validates each package against the rules declared in its matched preset: naming, tsconfig, eslint, required scripts, deps, files.

### `fix` — auto-fix violations

```bash
kb-devkit fix
kb-devkit fix --dry-run
kb-devkit fix --package @kb-labs/core
```

### `stats` — workspace health

```bash
kb-devkit stats           # health score A–F, issue counts by category
kb-devkit stats --json
```

### `status` — package table

```bash
kb-devkit status          # table: name, category, preset, issues
kb-devkit status --json
```

### `sync` — config sync

```bash
kb-devkit sync --check    # report drift without applying
kb-devkit sync --dry-run  # preview changes
kb-devkit sync            # apply
```

### `watch` — file watcher

```bash
kb-devkit watch --json    # stream violations as JSONL on file save
```

### `gate` — pre-commit gate

```bash
kb-devkit gate            # check staged files only; exits 1 on violations
```

### `doctor` — environment diagnostics

```bash
kb-devkit doctor --json
```

---

## Configuration (`devkit.yaml`)

### Categories

Categories classify packages and control which task variant runs for them. They are declared as an **ordered list** — the first matching category wins.

```yaml
workspace:
  categories:
    # More specific entries first — literal paths match before globs.
    spa:
      match:
        - "platform/kb-labs-studio/apps/studio"   # literal path, no wildcards
      preset: node-app

    ts-app:
      match:
        - "platform/*/apps/**"   # glob — would also match studio, but spa is declared first
      preset: node-app

    ts-lib:
      match:
        - "platform/*/packages/**"
      preset: node-lib

    go-binary:
      match:
        - "infra/kb-labs-devkit-bin"   # literal path — Go package without package.json
      preset: go-binary
```

**Matching rules:**
- Categories are evaluated top-to-bottom; first match wins — declaration order matters.
- Literal paths (no `*` or `**`) and glob patterns can be mixed freely in any order.
- Go packages or other non-JS directories don't need `package.json` — use a literal path in `match`.
- Packages that don't match any category are silently ignored by all commands.

### Task variants

Each task can have multiple variants — selected by package category. The scheduler picks the first variant whose `categories` list includes the package's category. Packages with no matching variant are silently skipped for that task.

```yaml
tasks:
  build:
    # TypeScript libraries and apps
    - categories: [ts-lib, ts-app]
      command: tsup
      inputs:
        - "src/**"
        - "tsup.config.ts"
        - "tsconfig*.json"
      outputs:
        - "dist/**"
      deps:
        - "^build"      # run 'build' for all workspace deps first

    # Go binaries
    - categories: [go-binary]
      command: make build
      inputs:
        - "**/*.go"
        - "go.mod"
        - "go.sum"
        - "Makefile"

    # Next.js sites
    - categories: [site]
      command: pnpm build
      inputs:
        - "app/**"
        - "components/**"
        - "next.config.*"
      outputs:
        - ".next/**"
      deps:
        - "^build"      # wait for ts-lib deps (e.g. @kb-labs/sdk)

  lint:
    - categories: [ts-lib, ts-app]
      command: eslint src/
      inputs: ["src/**", "eslint.config.*"]

    - categories: [site]
      command: eslint app/ components/
      inputs: ["app/**", "components/**", "eslint.config.*"]

  type-check:
    - categories: [ts-lib, ts-app, site]
      command: tsc --noEmit
      inputs: ["src/**", "tsconfig*.json"]
      deps: ["^build"]

  test:
    - categories: [ts-lib, ts-app]
      command: vitest run --passWithNoTests
      inputs: ["src/**", "test/**", "vitest.config.*"]
      outputs: ["coverage/**"]
      deps: ["build"]

  deploy:
    - command: ./scripts/deploy.sh   # no categories = applies to all
      inputs: ["dist/**"]
      cache: false                   # always runs, never cached
```

**Single variant (shorthand)** — if a task applies to all packages and needs no category filter, write it as a plain object:

```yaml
tasks:
  deploy:
    command: ./scripts/deploy.sh
    inputs: ["dist/**"]
    cache: false
```

### Dep syntax

| Value | Meaning |
|-------|---------|
| `^build` | Run `build` for every workspace dependency first (like Turbo `^`) |
| `build` | Run `build` for this same package first |

### `cache: false`

Set on tasks that must always run (e.g. deploy, publish). Result is not stored or restored from cache.

### Concurrency

```yaml
run:
  concurrency: 8   # max parallel (pkg, task) pairs; default: NumCPU-1
```

Override per-run with `--concurrency N`.

### Affected detection

```yaml
affected:
  strategy: submodules   # git | submodules | command
  # command: ./scripts/changed-files.sh
```

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
  init.go         ← kb-devkit init (generate starter devkit.yaml)
  run.go          ← kb-devkit run (task engine entry)
  check.go        ← kb-devkit check
  …

internal/
  cache/
    hash.go       ← InputHasher: globs → SHA256
    store.go      ← ObjectStore (LocalStore + interface for S3/R2)
    manifest.go   ← Manifest: inputHash → output refs + stdout
  engine/
    task.go       ← TaskDef, TaskResult, ResolveTaskDef(cfg, taskName, category)
    executor.go   ← hash → lookup → run → store
    scheduler.go  ← DAG builder (Kahn's), layer parallelism, AffectedPackages
  config/
    config.go     ← DevkitConfig, TaskConfig ([]TaskVariant), AffectedConfig, …
    yaml.go       ← YAML parsing — TaskVariant accepts single object or list
  workspace/
    workspace.go  ← Package (with Category field), Workspace, PackageByPath
    discover.go   ← glob-based package discovery
  checks/         ← individual quality check implementations
```
