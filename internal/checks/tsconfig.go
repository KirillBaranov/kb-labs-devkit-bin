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

// TSConfigRule checks tsconfig.json: must extend devkit preset, paths forbidden.
type TSConfigRule struct{}

func (r *TSConfigRule) Name() string { return "tsconfig" }

func (r *TSConfigRule) Check(pkg workspace.Package, preset config.Preset) []Issue {
	rules := preset.TSConfig
	if rules.MustExtendPattern == "" && !rules.ForbidPaths {
		return nil
	}

	path := filepath.Join(pkg.Dir, "tsconfig.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return []Issue{{
			Check:    r.Name(),
			Severity: SeverityError,
			Message:  "tsconfig.json not found",
			File:     path,
		}}
	}

	// tsconfig can have comments and trailing commas; use a lenient approach.
	var raw struct {
		Extends         string          `json:"extends"`
		CompilerOptions json.RawMessage `json:"compilerOptions"`
	}
	// Strip comments (naive: remove // lines) for basic parsing.
	cleaned := stripTSConfigComments(data)
	if err := json.Unmarshal(cleaned, &raw); err != nil {
		return []Issue{{
			Check:    r.Name(),
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("cannot parse tsconfig.json: %v", err),
			File:     path,
		}}
	}

	var issues []Issue

	// extends must contain the devkit pattern.
	if rules.MustExtendPattern != "" {
		if !strings.Contains(raw.Extends, rules.MustExtendPattern) {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("tsconfig.json extends %q but must extend a path matching %q", raw.Extends, rules.MustExtendPattern),
				File:     path,
				Fix:      fmt.Sprintf(`set "extends" to a path containing "%s"`, rules.MustExtendPattern),
			})
		}
	}

	// paths: forbidden (causes build order issues).
	if rules.ForbidPaths && len(raw.CompilerOptions) > 0 {
		var opts struct {
			Paths map[string]any `json:"paths"`
		}
		if err := json.Unmarshal(raw.CompilerOptions, &opts); err == nil && len(opts.Paths) > 0 {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  `compilerOptions.paths is forbidden; use workspace links instead`,
				File:     path,
				Fix:      "remove compilerOptions.paths from tsconfig.json",
			})
		}
	}

	return issues
}

// stripTSConfigComments removes // line comments for basic JSON parsing.
func stripTSConfigComments(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Remove inline comments (naive, doesn't handle // inside strings).
		if idx := strings.Index(line, "//"); idx > 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}
