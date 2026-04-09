package checks

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kb-labs/devkit/internal/config"
	"github.com/kb-labs/devkit/internal/workspace"
)

// StructureRule checks that required files exist in the package directory.
type StructureRule struct{}

func (r *StructureRule) Name() string { return "structure" }

func (r *StructureRule) Check(pkg workspace.Package, preset config.Preset) []Issue {
	if len(preset.Structure.RequiredFiles) == 0 {
		return nil
	}

	var issues []Issue
	for _, relFile := range preset.Structure.RequiredFiles {
		full := filepath.Join(pkg.Dir, relFile)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Check:    r.Name(),
				Severity: SeverityError,
				Message:  fmt.Sprintf("required file %q not found", relFile),
				File:     full,
				Fix:      fmt.Sprintf("create %s", relFile),
			})
		}
	}

	return issues
}
