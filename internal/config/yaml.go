package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func loadYAML(path string) (*DevkitConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	cfg := mapYAML(raw)
	return cfg, nil
}

// yamlConfig mirrors DevkitConfig for YAML tag parsing.
// Keeping it separate from the domain struct avoids leaking yaml tags.
type yamlConfig struct {
	Version   int                          `yaml:"version"`
	Workspace yamlWorkspace                `yaml:"workspace"`
	Sync      yamlSync                     `yaml:"sync"`
	Run       yamlRun                      `yaml:"run"`
	Tasks     map[string]yamlTask          `yaml:"tasks"`
	Affected  yamlAffected                 `yaml:"affected"`
	Presets   map[string]yamlPreset        `yaml:"presets"`
	Custom    []yamlCustomCheck            `yaml:"custom_checks"`
}

type yamlTask struct {
	Command string   `yaml:"command"`
	Inputs  []string `yaml:"inputs"`
	Outputs []string `yaml:"outputs"`
	Deps    []string `yaml:"deps"`
	Cache   *bool    `yaml:"cache"`
}

type yamlAffected struct {
	Strategy string `yaml:"strategy"`
	Command  string `yaml:"command"`
}

type yamlWorkspace struct {
	PackageManager string                      `yaml:"packageManager"`
	Categories     map[string]yamlCategory     `yaml:"categories"`
	MaxDepth       int                         `yaml:"maxDepth"`
}

type yamlCategory struct {
	Match    []string `yaml:"match"`
	Language string   `yaml:"language"`
	Preset   string   `yaml:"preset"`
}

type yamlPreset struct {
	Extends     string              `yaml:"extends"`
	Language    string              `yaml:"language"`
	PackageJSON yamlPackageJSON     `yaml:"package_json"`
	TSConfig    yamlTSConfig        `yaml:"tsconfig"`
	TSup        yamlTSup            `yaml:"tsup"`
	ESLint      yamlESLint          `yaml:"eslint"`
	Structure   yamlStructure       `yaml:"structure"`
	Deps        yamlDeps            `yaml:"dependencies"`
	Go          yamlGo              `yaml:"go"`
}

type yamlPackageJSON struct {
	RequiredScripts []string          `yaml:"required_scripts"`
	RequiredDevDeps map[string]string `yaml:"required_dev_deps"`
	RequiredFields  []string          `yaml:"required_fields"`
	Type            string            `yaml:"type"`
	Engines         map[string]string `yaml:"engines"`
}

type yamlTSConfig struct {
	MustExtendPattern string `yaml:"must_extend_pattern"`
	ForbidPaths       bool   `yaml:"forbid_paths"`
}

type yamlTSup struct {
	MustUseDevkitPreset bool   `yaml:"must_use_devkit_preset"`
	DTSRequired         bool   `yaml:"dts_required"`
	TSConfigMustBeBuild bool   `yaml:"tsconfig_must_be_build"`
	PresetPattern       string `yaml:"preset_pattern"`
}

type yamlESLint struct {
	MustUseDevkitPreset bool `yaml:"must_use_devkit_preset"`
}

type yamlStructure struct {
	RequiredFiles []string `yaml:"required_files"`
}

type yamlDeps struct {
	CheckLinks              bool `yaml:"check_links"`
	CheckUnused             bool `yaml:"check_unused"`
	CheckCircular           bool `yaml:"check_circular"`
	CheckVersionConsistency bool `yaml:"check_version_consistency"`
}

type yamlGo struct {
	MinVersion string `yaml:"min_version"`
}

type yamlSync struct {
	Sources map[string]yamlSyncSource `yaml:"sources"`
	Targets []yamlSyncTarget          `yaml:"targets"`
	Exclude []string                  `yaml:"exclude"`
}

type yamlSyncSource struct {
	Type    string `yaml:"type"`
	Package string `yaml:"package"`
	Path    string `yaml:"path"`
	URL     string `yaml:"url"`
	Ref     string `yaml:"ref"`
}

type yamlSyncTarget struct {
	Source string `yaml:"source"`
	From   string `yaml:"from"`
	To     string `yaml:"to"`
}

type yamlRun struct {
	Concurrency int `yaml:"concurrency"`
}

type yamlCustomCheck struct {
	Name     string   `yaml:"name"`
	Run      string   `yaml:"run"`
	On       []string `yaml:"on"`
	Language string   `yaml:"language"`
}

func mapYAML(raw yamlConfig) *DevkitConfig {
	cfg := &DevkitConfig{
		Version: raw.Version,
		Workspace: WorkspaceConfig{
			PackageManager: raw.Workspace.PackageManager,
			MaxDepth:       raw.Workspace.MaxDepth,
		},
		Run: RunConfig{
			Concurrency: raw.Run.Concurrency,
		},
		Affected: AffectedConfig{
			Strategy: raw.Affected.Strategy,
			Command:  raw.Affected.Command,
		},
	}

	// Categories
	if raw.Workspace.Categories != nil {
		cfg.Workspace.Categories = make(map[string]CategoryConfig, len(raw.Workspace.Categories))
		for k, v := range raw.Workspace.Categories {
			cfg.Workspace.Categories[k] = CategoryConfig{
				Match:    v.Match,
				Language: v.Language,
				Preset:   v.Preset,
			}
		}
	}

	// Presets
	if raw.Presets != nil {
		cfg.Presets = make(map[string]Preset, len(raw.Presets))
		for k, v := range raw.Presets {
			cfg.Presets[k] = mapPreset(v)
		}
	}

	// Sync
	cfg.Sync.Exclude = raw.Sync.Exclude
	if raw.Sync.Sources != nil {
		cfg.Sync.Sources = make(map[string]SyncSource, len(raw.Sync.Sources))
		for k, v := range raw.Sync.Sources {
			cfg.Sync.Sources[k] = SyncSource{
				Type:    v.Type,
				Package: v.Package,
				Path:    v.Path,
				URL:     v.URL,
				Ref:     v.Ref,
			}
		}
	}
	for _, t := range raw.Sync.Targets {
		cfg.Sync.Targets = append(cfg.Sync.Targets, SyncTarget{
			Source: t.Source,
			From:   t.From,
			To:     t.To,
		})
	}

	// Tasks
	if raw.Tasks != nil {
		cfg.Tasks = make(map[string]TaskConfig, len(raw.Tasks))
		for k, v := range raw.Tasks {
			cfg.Tasks[k] = TaskConfig{
				Command: v.Command,
				Inputs:  v.Inputs,
				Outputs: v.Outputs,
				Deps:    v.Deps,
				Cache:   v.Cache,
			}
		}
	}

	// Custom checks
	for _, c := range raw.Custom {
		cfg.Custom = append(cfg.Custom, CustomCheck{
			Name:     c.Name,
			Run:      c.Run,
			On:       c.On,
			Language: c.Language,
		})
	}

	return cfg
}

func mapPreset(v yamlPreset) Preset {
	p := Preset{
		Extends:  v.Extends,
		Language: v.Language,
		PackageJSON: PackageJSONRules{
			RequiredScripts: v.PackageJSON.RequiredScripts,
			RequiredDevDeps: v.PackageJSON.RequiredDevDeps,
			RequiredFields:  v.PackageJSON.RequiredFields,
			Type:            v.PackageJSON.Type,
			Engines:         v.PackageJSON.Engines,
		},
		TSConfig: TSConfigRules{
			MustExtendPattern: v.TSConfig.MustExtendPattern,
			ForbidPaths:       v.TSConfig.ForbidPaths,
		},
		TSup: TSupRules{
			MustUseDevkitPreset: v.TSup.MustUseDevkitPreset,
			DTSRequired:         v.TSup.DTSRequired,
			TSConfigMustBeBuild: v.TSup.TSConfigMustBeBuild,
			PresetPattern:       v.TSup.PresetPattern,
		},
		ESLint: ESLintRules{
			MustUseDevkitPreset: v.ESLint.MustUseDevkitPreset,
		},
		Structure: StructureRules{
			RequiredFiles: v.Structure.RequiredFiles,
		},
		Deps: DepsRules{
			CheckLinks:              v.Deps.CheckLinks,
			CheckUnused:             v.Deps.CheckUnused,
			CheckCircular:           v.Deps.CheckCircular,
			CheckVersionConsistency: v.Deps.CheckVersionConsistency,
		},
		Go: GoRules{
			MinVersion: v.Go.MinVersion,
		},
	}
	return p
}
