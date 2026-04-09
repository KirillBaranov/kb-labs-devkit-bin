package checks

import "github.com/kb-labs/devkit/internal/config"

var defaultRules []Rule

func init() {
	defaultRules = []Rule{
		&PackageJSONRule{},
		&TSConfigRule{},
		&TSupRule{},
		&ESLintRule{},
		&StructureRule{},
		&DepsRule{},
	}
}

// Registry holds registered rules.
type Registry struct {
	rules []Rule
}

// Default returns a Registry with all built-in rules registered.
func Default() *Registry {
	r := &Registry{}
	r.rules = append(r.rules, defaultRules...)
	return r
}

// Register adds a custom rule to the registry.
func (r *Registry) Register(rule Rule) {
	r.rules = append(r.rules, rule)
}

// RulesFor returns rules applicable to a given preset's language.
func (r *Registry) RulesFor(preset config.Preset) []Rule {
	if preset.Language == "go" {
		// Go packages: only structure rule applies (+ go-specific in future)
		var result []Rule
		for _, rule := range r.rules {
			if rule.Name() == "structure" {
				result = append(result, rule)
			}
		}
		return result
	}
	// TypeScript packages: all rules.
	return r.rules
}

// All returns all registered rules.
func (r *Registry) All() []Rule {
	return r.rules
}
