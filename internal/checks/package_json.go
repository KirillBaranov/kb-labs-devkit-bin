package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// semverCompatible returns true if gotVer satisfies the wantVer range.
// Handles the common monorepo case: want "^5", got "^5.6.3" → compatible.
// Rule: if both have the same caret prefix and the same major version, it's compatible.
func semverCompatible(got, want string) bool {
	if got == want {
		return true
	}
	// Strip caret/tilde prefix for comparison.
	gotBase := strings.TrimLeft(got, "^~>=<")
	wantBase := strings.TrimLeft(want, "^~>=<")

	// Extract major version (first segment before ".").
	gotMajor := strings.SplitN(gotBase, ".", 2)[0]
	wantMajor := strings.SplitN(wantBase, ".", 2)[0]

	// If majors match and the range prefix is the same, compatible.
	gotPrefix := strings.TrimRight(got, "0123456789.-")
	wantPrefix := strings.TrimRight(want, "0123456789.-")
	return gotMajor == wantMajor && gotPrefix == wantPrefix
}

// PackageJSONRule checks package.json for required scripts, devDeps, fields, type, and engines.
type PackageJSONRule struct{}

func (r *PackageJSONRule) Name() string { return "package_json" }

func (r *PackageJSONRule) Check(pkg workspace.Package, preset config.Preset) []Issue {
	rules := preset.PackageJSON
	if len(rules.RequiredScripts) == 0 && len(rules.RequiredDevDeps) == 0 && len(rules.RequiredFields) == 0 {
		return nil
	}

	path := filepath.Join(pkg.Dir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Check:    r.Name(),
			Severity: SeverityError,
			Message:  "package.json not found",
			File:     path,
		}}
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return []Issue{{
			Check:    r.Name(),
			Severity: SeverityError,
			Message:  fmt.Sprintf("invalid JSON: %v", err),
			File:     path,
		}}
	}

	var issues []Issue

	// Required top-level fields.
	for _, field := range rules.RequiredFields {
		if _, ok := raw[field]; !ok {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf(`missing required field %q`, field),
				File:     path,
			})
		}
	}

	// type: module
	if rules.Type != "" {
		var pkgType string
		if t, ok := raw["type"]; ok {
			_ = json.Unmarshal(t, &pkgType)
		}
		if pkgType != rules.Type {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf(`"type" must be %q, got %q`, rules.Type, pkgType),
				File:     path,
				Fix:      fmt.Sprintf(`add "type": "%s" to package.json`, rules.Type),
			})
		}
	}

	// Required scripts.
	var scripts map[string]string
	if s, ok := raw["scripts"]; ok {
		_ = json.Unmarshal(s, &scripts)
	}
	for _, script := range rules.RequiredScripts {
		if _, ok := scripts[script]; !ok {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("missing required script %q", script),
				File:     path,
				Fix:      fmt.Sprintf(`add script "%s" to package.json`, script),
			})
		}
	}

	// Required devDependencies — "more is OK, less is not".
	var devDeps map[string]string
	if d, ok := raw["devDependencies"]; ok {
		_ = json.Unmarshal(d, &devDeps)
	}
	for dep, wantVer := range rules.RequiredDevDeps {
		gotVer, ok := devDeps[dep]
		if !ok {
			// Also check dependencies (some tools put devkit there).
			var deps map[string]string
			if d, ok2 := raw["dependencies"]; ok2 {
				_ = json.Unmarshal(d, &deps)
			}
			gotVer, ok = deps[dep]
		}
		if !ok {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("missing required devDependency %q", dep),
				File:     path,
				Fix:      fmt.Sprintf(`add "%s": "%s" to devDependencies`, dep, wantVer),
			})
			continue
		}
		// Wildcard "*" means any version is OK.
		if wantVer != "*" && !strings.HasPrefix(gotVer, "link:") && !strings.HasPrefix(gotVer, "workspace:") {
			if !semverCompatible(gotVer, wantVer) {
				issues = append(issues, Issue{
					Check:    r.Name(),
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("devDependency %q version %q does not match expected %q", dep, gotVer, wantVer),
					File:     path,
				})
			}
		}
	}

	// Engines — "more is OK, less is not" (required engines must be present).
	var engines map[string]string
	if e, ok := raw["engines"]; ok {
		_ = json.Unmarshal(e, &engines)
	}
	for eng, wantVer := range rules.Engines {
		if _, ok := engines[eng]; !ok {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("missing required engine %q: %q in engines field", eng, wantVer),
				File:     path,
				Fix:      fmt.Sprintf(`add "%s": "%s" to engines`, eng, wantVer),
			})
		}
	}

	return issues
}
