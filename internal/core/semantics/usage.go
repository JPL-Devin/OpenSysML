package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// UsageMayTimeVary derives SysML Usage::mayTimeVary (SysML v2 §8.3.6.4).
func (m *Model) UsageMayTimeVary(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !m.conformsByName(owner, "Occurrences::Occurrence") ||
		usage.IsPortion || usage.Portion != ast.PortionNone ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !usageIsComposite(usage) ||
		(usage.Kind != ast.UsageAction && !m.conformsByName(sym, "Actions::Action"))
}

func usageIsComposite(usage *ast.Usage) bool {
	if usage == nil || usage.IsReference || usage.Direction != ast.DirNone ||
		usage.IsEnd || usage.IsEvent {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			return false
		}
	}
	return true
}
