package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// category is the SysML v2 declaration a v1 classifier or package becomes.
type category int

const (
	catNone category = iota
	catPackage
	catPartDef
	catPortDef
	catAttributeDef
	catEnumDef
	catConstraintDef
	catRequirementDef
	catConnectionDef
	catIndividualDef
	catVerificationDef
	catItemDef
	// catLibrary marks profile and bundled-library content that is not migrated.
	catLibrary
	// catUnmapped marks a classifier this migration has no v2 form for.
	catUnmapped
)

// keyword is the v2 declaration keyword of a category; "" for none.
func (c category) keyword() string {
	switch c {
	case catPackage:
		return "package"
	case catPartDef:
		return "part def"
	case catPortDef:
		return "port def"
	case catAttributeDef:
		return "attribute def"
	case catEnumDef:
		return "enum def"
	case catConstraintDef:
		return "constraint def"
	case catRequirementDef:
		return "requirement def"
	case catConnectionDef:
		return "connection def"
	case catIndividualDef:
		return "individual def"
	case catVerificationDef:
		return "verification def"
	case catItemDef:
		return "item def"
	}
	return ""
}

// requirementStereotypes are the SysML requirement stereotype and the
// specialized requirement kinds MagicDraw's SysML customization adds.
var requirementStereotypes = []string{
	"Requirement", "AbstractRequirement", "extendedRequirement", "functionalRequirement",
	"performanceRequirement", "physicalRequirement", "interfaceRequirement", "designConstraint",
	"usabilityRequirement", "businessRequirement",
}

// scalarValues maps the v1 primitive value type names to the ScalarValues
// library types they correspond to.
var scalarValues = map[string]string{
	"Real":             "Real",
	"Integer":          "Integer",
	"Boolean":          "Boolean",
	"String":           "String",
	"UnlimitedNatural": "Natural",
	"Natural":          "Natural",
	"Complex":          "Complex",
	"Number":           "Number",
	"Rational":         "Rational",
}

// libraryRoots are the names of the profile and library packages a Cameo export
// carries alongside the user's model.
var libraryRoots = map[string]bool{
	"SysML":                             true,
	"UML Standard Profile":              true,
	"MD Customization for SysML":        true,
	"MD Customization for Requirements": true,
	"QUDV":                              true,
	"ISO-80000":                         true,
	"SI Definitions":                    true,
	"SIDefinitions":                     true,
	"SI Value Type Library":             true,
	"SI Specializations":                true,
	"MagicDraw Profile":                 true,
	"PrimitiveTypes":                    true,
	"PrimitiveValueTypes":               true,
	"Libraries":                         true,
}

// isLibrary reports whether e sits in profile or bundled-library content.
func isLibrary(e *xmi.Element) bool {
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Type == "Profile" {
			return true
		}
		if cur.Parent == nil || cur.Parent.Type == "Model" || cur.Parent.Parent == nil {
			if libraryRoots[cur.Name] {
				return true
			}
		}
	}
	return false
}

// scalarValue returns the ScalarValues type a v1 type maps to, or "" when the
// type is the user's own: a primitive of the UML or SysML libraries by name,
// whether referenced by href or bundled in the document.
func scalarValue(t *xmi.Element) string {
	if t == nil {
		return ""
	}
	name, ok := scalarValues[t.Name]
	if !ok {
		return ""
	}
	if t.IsProxy() {
		return name
	}
	if t.Type == "PrimitiveType" {
		return name
	}
	if t.Type == "DataType" && isLibrary(t) {
		return name
	}
	return ""
}

// classify decides which v2 declaration a classifier or package becomes, and
// why the choice is only an approximation when it is.
func (m *migration) classify(e *xmi.Element) (category, string) {
	if e.IsProxy() {
		return catNone, ""
	}
	if isLibrary(e) {
		return catLibrary, ""
	}
	switch e.Type {
	case "Model", "Package":
		return catPackage, ""
	case "Profile":
		return catLibrary, ""
	case "Class", "Component":
		switch {
		case e.HasStereotype(requirementStereotypes...):
			return catRequirementDef, ""
		case e.HasStereotype("ConstraintBlock"):
			return catConstraintDef, ""
		case e.HasStereotype("InterfaceBlock"):
			return catPortDef, ""
		case e.HasStereotype("Block"):
			return catPartDef, ""
		case e.HasStereotype("Stakeholder"):
			return catPartDef, "a v1 «Stakeholder» is written as a part def"
		case e.HasStereotype("View"):
			return catUnmapped, "views are not migrated yet"
		case e.HasStereotype("Viewpoint"):
			return catUnmapped, "viewpoints are not migrated yet"
		}
		return catPartDef, "a plain UML class without «Block» is written as a part def"
	case "Actor":
		return catPartDef, "a UML actor is written as a part def"
	case "AssociationClass":
		return catConnectionDef, ""
	case "Association":
		return catConnectionDef, ""
	case "DataType":
		if e.HasStereotype("ValueType") {
			return catAttributeDef, ""
		}
		return catAttributeDef, "a UML data type without «ValueType» is written as an attribute def"
	case "PrimitiveType":
		return catAttributeDef, ""
	case "Enumeration":
		return catEnumDef, ""
	case "Signal":
		return catAttributeDef, "a signal is written as an attribute def; v2 has no signal classifier"
	case "Interface":
		return catPortDef, "a UML interface is written as a port def"
	case "InstanceSpecification":
		if e.HasStereotype("Unit", "QuantityKind") {
			return catUnmapped, "units and quantity kinds are not migrated; use the SI and ISQ libraries"
		}
		if m.model.Ref(e, "classifier") == nil {
			return catUnmapped, "an instance specification without a classifier has no v2 form"
		}
		return catIndividualDef, ""
	case "Activity", "OpaqueBehavior", "Interaction", "StateMachine", "FunctionBehavior":
		if e.HasStereotype("TestCase") {
			return catVerificationDef, "the test case's behavior is not migrated; only its verified requirements are"
		}
		return catUnmapped, "behaviors are not migrated yet"
	case "UseCase":
		return catUnmapped, "use cases are not migrated yet"
	case "Collaboration", "Node", "Device", "ExecutionEnvironment", "Artifact":
		return catUnmapped, "no v2 form for a UML " + e.Type
	}
	return catUnmapped, "no v2 form for a UML " + e.Type
}

// kindOf names the v1 element as its author saw it: its classifying
// stereotype in guillemets, else its UML metaclass.
func kindOf(e *xmi.Element) string {
	for _, s := range e.Stereotypes {
		switch s.Name {
		case "PartProperty", "ValueProperty", "ReferenceProperty", "SharedProperty", "ConstraintProperty", "ConstraintParameter":
			// MagicDraw's property customizations restate what the type says.
			continue
		}
		return "«" + s.Name + "»" + " " + e.Type
	}
	return e.Type
}

// qualifiedName joins the v1 names from below the root model to e.
func qualifiedName(e *xmi.Element) string {
	path := e.Path()
	if len(path) > 1 && rootOf(e).Type == "Model" {
		path = path[1:]
	}
	return strings.Join(path, "::")
}

func rootOf(e *xmi.Element) *xmi.Element {
	for e.Parent != nil {
		e = e.Parent
	}
	return e
}
