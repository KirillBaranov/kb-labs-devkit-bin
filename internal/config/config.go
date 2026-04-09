// Package config loads and resolves devkit.yaml configuration.
package config

// DevkitConfig is the top-level structure for devkit.yaml.
type DevkitConfig struct {
	Version   int                        `yaml:"version"`
	Workspace WorkspaceConfig            `yaml:"workspace"`
	Sync      SyncConfig                 `yaml:"sync"`
	Build     BuildConfig                `yaml:"build"`
	Tasks     map[string]TaskConfig      `yaml:"tasks"`
	Presets   map[string]Preset          `yaml:"presets"`
	Custom    []CustomCheck              `yaml:"custom_checks"`
}

// TaskConfig defines a named task in devkit.yaml.
// Tasks are the unit of cached execution: build, lint, test, deploy, etc.
type TaskConfig struct {
	Command string   `yaml:"command"`
	Inputs  []string `yaml:"inputs"`  // glob patterns relative to package dir
	Outputs []string `yaml:"outputs"` // glob patterns; empty = no output files (cache exit code)
	Deps    []string `yaml:"deps"`    // "^build" = deps' task first; "build" = self's task first
	Cache   *bool    `yaml:"cache"`   // nil = true; false = always run (e.g. deploy)
}

// WorkspaceConfig describes package categories and the package manager.
type WorkspaceConfig struct {
	PackageManager string                     `yaml:"packageManager"`
	Categories     map[string]CategoryConfig  `yaml:"categories"`
	// MaxDepth controls how deep to recurse when expanding ** globs.
	// Default: 3. Increase if your monorepo has deeply nested packages.
	MaxDepth int `yaml:"maxDepth"`
}

// CategoryConfig matches packages to a preset by glob patterns.
type CategoryConfig struct {
	Match    []string `yaml:"match"`
	Language string   `yaml:"language"`
	Preset   string   `yaml:"preset"`
}

// PresetConfig resolves the preset for this category from the given config.
func (c CategoryConfig) PresetConfig(cfg *DevkitConfig) (Preset, error) {
	return ResolvePreset(c.Preset, cfg)
}

// ─── Presets ────────────────────────────────────────────────────────────────

// Preset is a named ruleset for a category of packages.
type Preset struct {
	Extends     string           `yaml:"extends"`
	Language    string           `yaml:"language"`
	PackageJSON PackageJSONRules `yaml:"package_json"`
	TSConfig    TSConfigRules    `yaml:"tsconfig"`
	TSup        TSupRules        `yaml:"tsup"`
	ESLint      ESLintRules      `yaml:"eslint"`
	Structure   StructureRules   `yaml:"structure"`
	Deps        DepsRules        `yaml:"dependencies"`
	Go          GoRules          `yaml:"go"`
}

// PackageJSONRules defines requirements for package.json.
type PackageJSONRules struct {
	RequiredScripts  []string          `yaml:"required_scripts"`
	RequiredDevDeps  map[string]string `yaml:"required_dev_deps"`
	RequiredFields   []string          `yaml:"required_fields"`
	Type             string            `yaml:"type"`
	Engines          map[string]string `yaml:"engines"`
}

// TSConfigRules defines requirements for tsconfig.json.
type TSConfigRules struct {
	MustExtendPattern string `yaml:"must_extend_pattern"`
	ForbidPaths       bool   `yaml:"forbid_paths"`
}

// TSupRules defines requirements for tsup.config.ts.
type TSupRules struct {
	MustUseDevkitPreset  bool   `yaml:"must_use_devkit_preset"`
	DTSRequired          bool   `yaml:"dts_required"`
	TSConfigMustBeBuild  bool   `yaml:"tsconfig_must_be_build"`
	PresetPattern        string `yaml:"preset_pattern"`
}

// ESLintRules defines requirements for eslint.config.js.
type ESLintRules struct {
	MustUseDevkitPreset bool `yaml:"must_use_devkit_preset"`
}

// StructureRules defines required files that must exist.
type StructureRules struct {
	RequiredFiles []string `yaml:"required_files"`
}

// DepsRules defines dependency analysis rules.
type DepsRules struct {
	CheckLinks           bool `yaml:"check_links"`
	CheckUnused          bool `yaml:"check_unused"`
	CheckCircular        bool `yaml:"check_circular"`
	CheckVersionConsistency bool `yaml:"check_version_consistency"`
}

// GoRules defines rules for Go packages.
type GoRules struct {
	MinVersion string `yaml:"min_version"`
}

// ─── Sync ────────────────────────────────────────────────────────────────────

// SyncConfig describes sync sources and targets.
type SyncConfig struct {
	Sources map[string]SyncSource `yaml:"sources"`
	Targets []SyncTarget          `yaml:"targets"`
	Exclude []string              `yaml:"exclude"`
}

// SyncSource describes where assets come from.
type SyncSource struct {
	Type    string `yaml:"type"`    // npm | local | git | url
	Package string `yaml:"package"` // for type: npm
	Path    string `yaml:"path"`    // for type: local
	URL     string `yaml:"url"`     // for type: git | url
	Ref     string `yaml:"ref"`     // for type: git
}

// SyncTarget maps a source asset to a destination in each submodule.
type SyncTarget struct {
	Source string `yaml:"source"` // key from Sources
	From   string `yaml:"from"`   // path within source FS
	To     string `yaml:"to"`     // destination path relative to submodule root
}

// ─── Build ───────────────────────────────────────────────────────────────────

// BuildConfig describes the build runner configuration.
type BuildConfig struct {
	Runner      string `yaml:"runner"`      // native | turbo | custom
	Command     string `yaml:"command"`     // for runner: custom
	Cache       bool   `yaml:"cache"`
	Concurrency int    `yaml:"concurrency"`
}

// ─── Custom checks ────────────────────────────────────────────────────────────

// CustomCheck is an escape-hatch checker executed as a shell command.
type CustomCheck struct {
	Name     string   `yaml:"name"`
	Run      string   `yaml:"run"`
	On       []string `yaml:"on"`       // check | gate
	Language string   `yaml:"language"` // any | typescript | go
}
