# kb-devkit

Go binary for workspace quality management. Enforces standards, syncs configs, and manages builds across all packages in a monorepo.

## Build

```bash
make build
```

## Usage

```bash
kb-devkit check                          # check all packages
kb-devkit check --package @kb-labs/core # check single package
kb-devkit check --json                  # machine-readable output

kb-devkit status                        # workspace health table
kb-devkit status --json

kb-devkit sync --check                  # report config drift
kb-devkit sync --dry-run                # preview sync
kb-devkit sync                          # apply sync

kb-devkit build                         # native runner
kb-devkit build --runner turbo          # turbo
kb-devkit build --affected              # only changed packages

kb-devkit watch --json                  # stream violations as JSONL

kb-devkit gate                          # pre-commit gate (staged files only)

kb-devkit deps --circular               # detect circular dependencies

kb-devkit doctor --json                 # environment diagnostics

kb-devkit run ./scripts/my-check.sh    # custom checker
```

## Configuration

Place `devkit.yaml` in your workspace root. See the schema in `internal/config/config.go`.

## Data plane

Config templates and assets live in `@kb-labs/devkit` (npm package in `infra/kb-labs-devkit`).
This binary is the execution plane — it reads devkit.yaml and applies the rules.
