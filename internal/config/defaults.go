package config

// defaultPresets contains built-in presets as Go struct literals.
// These are used when devkit.yaml doesn't define custom presets,
// or as the base when a preset uses extends:.
var defaultPresets = map[string]Preset{
	"node-lib": {
		Language: "typescript",
		PackageJSON: PackageJSONRules{
			RequiredScripts: []string{"build", "dev", "test", "lint", "type-check", "clean"},
			RequiredDevDeps: map[string]string{
				"typescript":      "^5",
				"tsup":            "^8",
				"vitest":          "^3",
				"@kb-labs/devkit": "*",
			},
			RequiredFields: []string{"name", "version", "type", "engines", "exports"},
			Type:           "module",
			Engines: map[string]string{
				"node": ">=20.0.0",
				"pnpm": ">=9.0.0",
			},
		},
		TSConfig: TSConfigRules{
			MustExtendPattern: "@kb-labs/devkit/tsconfig/",
			ForbidPaths:       true,
		},
		TSup: TSupRules{
			MustUseDevkitPreset: true,
			DTSRequired:         true,
			TSConfigMustBeBuild: true,
		},
		ESLint: ESLintRules{
			MustUseDevkitPreset: true,
		},
		Structure: StructureRules{
			RequiredFiles: []string{"src/index.ts", "README.md"},
		},
		Deps: DepsRules{
			CheckLinks:              true,
			CheckUnused:             false, // too noisy by default
			CheckCircular:           true,
			CheckVersionConsistency: true,
		},
	},

	"node-cli": {
		Extends:  "node-lib",
		Language: "typescript",
		TSup: TSupRules{
			MustUseDevkitPreset: true,
			DTSRequired:         true,
			TSConfigMustBeBuild: true,
			PresetPattern:       "binPreset|cliPreset",
		},
	},

	"go-binary": {
		Language: "go",
		Structure: StructureRules{
			RequiredFiles: []string{"go.mod", "Makefile"},
		},
		Go: GoRules{
			MinVersion: "1.21",
		},
	},

	"site": {
		Language: "typescript",
		PackageJSON: PackageJSONRules{
			RequiredScripts: []string{"build", "dev"},
		},
	},
}

// GetDefaultPreset returns the built-in preset by name, or nil if not found.
func GetDefaultPreset(name string) (Preset, bool) {
	p, ok := defaultPresets[name]
	return p, ok
}
