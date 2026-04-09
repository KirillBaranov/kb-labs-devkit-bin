package workspace

import (
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/kb-labs/devkit/internal/config"
)

// discoverPackages reads pnpm-workspace.yaml, expands globs, finds package.json files,
// and classifies each package against the categories in cfg.
func discoverPackages(root string, cfg *config.DevkitConfig) ([]Package, error) {
	patterns, err := readPnpmWorkspace(root)
	if err != nil {
		// No pnpm-workspace.yaml: try to fall back to package.json workspaces field.
		patterns, err = readPackageJSONWorkspaces(root)
		if err != nil {
			return nil, err
		}
	}

	var pkgDirs []string
	fsys := os.DirFS(root)

	for _, pattern := range patterns {
		// Pattern may be "packages/*" or "platform/*/packages/*" etc.
		// We match against package.json inside each dir.
		globPattern := pattern + "/package.json"
		matches, err := doublestar.Glob(fsys, globPattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			dir := filepath.Join(root, filepath.Dir(match))
			pkgDirs = append(pkgDirs, dir)
		}
	}

	return classifyPackages(root, pkgDirs, cfg), nil
}

// classifyPackages maps each package dir to a category+preset.
func classifyPackages(root string, dirs []string, cfg *config.DevkitConfig) []Package {
	var result []Package
	for _, dir := range dirs {
		rel, _ := filepath.Rel(root, dir)
		name := readPackageName(dir)

		category, preset, language := classify(rel, cfg)
		if category == "" {
			// Uncategorized — skip silently.
			continue
		}

		result = append(result, Package{
			Name:     name,
			Dir:      dir,
			Category: category,
			Preset:   preset,
			Language: language,
			RelPath:  rel,
		})
	}
	return result
}

// classify returns the category, preset, and language for a package relative path.
// First match wins (order of categories in devkit.yaml is significant).
func classify(relPath string, cfg *config.DevkitConfig) (category, preset, language string) {
	if cfg == nil || cfg.Workspace.Categories == nil {
		return "", "", ""
	}

	// YAML maps are unordered; we need a stable order.
	// For now we iterate all and return first match.
	// TODO: if ordering matters in practice, store categories as []struct in config.
	for catName, cat := range cfg.Workspace.Categories {
		for _, pattern := range cat.Match {
			matched, err := doublestar.Match(pattern, relPath)
			if err != nil {
				continue
			}
			if matched {
				lang := cat.Language
				if lang == "" {
					lang = inferLanguage(cat.Preset)
				}
				return catName, cat.Preset, lang
			}
		}
	}
	return "", "", ""
}

func inferLanguage(preset string) string {
	switch preset {
	case "go-binary":
		return "go"
	default:
		return "typescript"
	}
}
