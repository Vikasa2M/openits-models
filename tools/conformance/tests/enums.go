package tests

import (
	"strings"
	"sync"

	"github.com/openconfig/ygot/ygot"

	yangpkg "github.com/Vikasa2M/openits-models/pkg/yang/openits"
)

// Helpers shared by the checks that test a module-qualified identity
// string carried on a shared event (a fault kind, a mode value) against a
// generated enum.

var (
	knownModulesOnce sync.Once
	knownModules     map[string]bool
)

// generatedModules is the set of modules that define at least one
// identity or enum in the generated schema. A value from an openits-*
// module outside this set is reported as an unknown module (a typo),
// except openits-vendor-* modules, which are not generator inputs and are
// skipped.
func generatedModules() map[string]bool {
	knownModulesOnce.Do(func() {
		knownModules = map[string]bool{}
		for _, defs := range yangpkg.ΛEnum {
			for _, d := range defs {
				if d.DefiningModule != "" {
					knownModules[d.DefiningModule] = true
				}
			}
		}
	})
	return knownModules
}

// enumMembers returns the generated definitions of the named enum.
func enumMembers(enum string) map[int64]ygot.EnumDefinition {
	return yangpkg.ΛEnum[enum]
}

// enumHas reports whether the named generated enum has a member with this
// identity name and defining module.
func enumHas(enum, module, name string) bool {
	for _, d := range enumMembers(enum) {
		if d.Name == name && d.DefiningModule == module {
			return true
		}
	}
	return false
}

// identityStatus classifies a module-qualified identity string.
type identityStatus int

const (
	identityChecked       identityStatus = iota // from a generated openits module
	identityEmpty                               // the empty string
	identityNoPrefix                            // no "<module>:" prefix
	identitySkipped                             // a vendor or non-openits module
	identityUnknownModule                       // an openits module the generated schema does not know
)

// classifyIdentity splits value into module and name and says whether a
// check can test it against the generated enums.
func classifyIdentity(value string) (module, name string, st identityStatus) {
	if value == "" {
		return "", "", identityEmpty
	}
	module, name, found := strings.Cut(value, ":")
	if !found {
		return "", value, identityNoPrefix
	}
	if !strings.HasPrefix(module, "openits-") || strings.HasPrefix(module, "openits-vendor-") {
		return module, name, identitySkipped
	}
	if !generatedModules()[module] {
		return module, name, identityUnknownModule
	}
	return module, name, identityChecked
}
