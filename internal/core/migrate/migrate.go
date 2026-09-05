// Package migrate writes a SysML v1 model, read from XMI, as SysML v2 textual
// notation, and reports what each v1 element became.
package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// Result is a migration's output: the v2 notation and the report over it.
type Result struct {
	Notation []byte
	Report   *Report
}

// Migrate reads a Cameo/MagicDraw XMI document or .mdzip archive and writes it
// as SysML v2 notation. name labels the source in the report.
func Migrate(name string, data []byte) (*Result, error) {
	model, err := xmi.Parse(data)
	if err != nil {
		return nil, err
	}
	return FromModel(name, model), nil
}

// FromModel migrates an already-read XMI model.
func FromModel(name string, model *xmi.Model) *Result {
	m := &migration{
		model:    model,
		report:   &Report{Source: name, Exporter: model.Exporter},
		w:        &writer{},
		names:    map[*xmi.Element]string{},
		extras:   map[*xmi.Element][]func(){},
		flows:    map[*xmi.Element][]*xmi.Element{},
		unplaced: map[*xmi.Element]string{},
	}
	m.prepare()
	for _, root := range model.Roots {
		m.root(root)
	}
	m.extensions()
	return &Result{Notation: []byte(m.w.String()), Report: m.report}
}

// extensions accounts for the diagrams and other tool-private content the
// reader skipped, so a report never loses them silently.
func (m *migration) extensions() {
	for _, ext := range m.model.Extensions {
		note := "tool-private xmi:Extension content"
		if ext.Extender != "" {
			note += " written by " + ext.Extender
		}
		for _, el := range ext.Elements {
			name := el.Name
			if name == "" {
				name = "<" + el.Type + ">"
			}
			if ext.Owner != nil && ext.Owner.Parent != nil {
				name = qualifiedName(ext.Owner) + "::" + name
			}
			kind := strings.TrimPrefix(el.Type, "uml:")
			m.report.Entries = append(m.report.Entries, Entry{ID: el.ID, Kind: kind, Name: name, Verdict: Skipped, Note: note})
		}
	}
}

// migration holds the state of one run.
type migration struct {
	model  *xmi.Model
	report *Report
	w      *writer
	// names holds the names synthesized for anonymous elements.
	names map[*xmi.Element]string
	// extras are members other elements contribute to a body: a Satisfy is
	// written inside the block that satisfies.
	extras map[*xmi.Element][]func()
	// flows lists the item flows each connector realizes.
	flows map[*xmi.Element][]*xmi.Element
	// unplaced records why a Satisfy or Verify could not be placed in a body.
	unplaced map[*xmi.Element]string
	// scope is the element whose body is being written; nil at the top level.
	scope *xmi.Element
}

func (m *migration) add(e *xmi.Element, v Verdict, target, note string) {
	m.report.Entries = append(m.report.Entries, Entry{
		ID: e.ID, Kind: kindOf(e), Name: qualifiedName(e), Target: target, Verdict: v, Note: note,
	})
}

// prepare walks the model once ahead of writing: it indexes the item flows
// by realizing connector and names every anonymous feature that is referred to.
func (m *migration) prepare() {
	var walk func(e *xmi.Element)
	walk = func(e *xmi.Element) {
		switch e.Type {
		case "InformationFlow":
			for _, c := range m.model.Refs(e, "realizingConnector") {
				m.flows[c] = append(m.flows[c], e)
			}
		case "ConnectorEnd":
			for _, role := range []string{"role", "partWithPort"} {
				if r := m.model.Ref(e, role); r != nil {
					m.nameFor(r)
				}
			}
			if nce := e.Stereotype("NestedConnectorEnd"); nce != nil {
				for _, id := range nce.Tags["propertyPath"] {
					if p := m.model.Lookup(id); p != nil {
						m.nameFor(p)
					}
				}
			}
		case "Dependency", "Abstraction", "Realization", "Usage":
			for _, role := range []string{"client", "supplier"} {
				if r := m.model.Ref(e, role); r != nil && (r.Type == "Property" || r.Type == "Port") {
					m.nameFor(r)
				}
			}
			m.placeDependency(e)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, r := range m.model.Roots {
		if !isLibrary(r) {
			walk(r)
		}
	}
}

// root writes a top-level element: a Model's members are written at the top
// level, any other root as a declaration of its own.
func (m *migration) root(e *xmi.Element) {
	if e.Type == "Model" && !isLibrary(e) {
		m.add(e, Mapped, "", "the root model's members are written at the top level")
		m.body(e)
		return
	}
	m.member(e)
}

// body writes the members of e's body, in document order, then what other
// elements contribute to it.
func (m *migration) body(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	for _, c := range e.Children {
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.scope = saved
}

// member writes one owned element of the current scope.
func (m *migration) member(e *xmi.Element) {
	switch e.Role {
	case "ownedComment", "generalization", "lowerValue", "upperValue", "defaultValue",
		"end", "specification", "type", "general", "annotatedElement", "body", "language",
		"ownedEnd", "memberEnd", "value", "slot", "ownedLiteral", "ownedParameter", "region":
		// Written by the owner.
		return
	case "profileApplication", "packageImport", "elementImport", "packageMerge":
		m.imports(e)
		return
	case "ownedAttribute":
		m.feature(e)
		return
	case "ownedConnector":
		m.connector(e)
		return
	case "ownedRule":
		m.rule(e)
		return
	case "ownedOperation", "ownedReception":
		m.unmapped(e, "operations and receptions are not migrated; v2 has no operation")
		return
	}
	switch e.Type {
	case "Dependency", "Abstraction", "Realization", "Usage":
		m.dependency(e)
		return
	case "InformationFlow":
		m.informationFlow(e)
		return
	case "Comment":
		// A comment in a non-ownedComment role is still a comment.
		m.comment(e)
		return
	}
	m.classifier(e)
}

// imports writes a package import; profile applications and element imports
// have no v2 counterpart worth writing.
func (m *migration) imports(e *xmi.Element) {
	if e.Type != "PackageImport" {
		m.add(e, Skipped, "", "profile applications and element imports are not written")
		return
	}
	target := m.model.Ref(e, "importedPackage")
	if target == nil || target.IsProxy() || isLibrary(target) {
		m.add(e, Skipped, "", "import of a profile or library package")
		return
	}
	vis := ""
	if e.Attrs["visibility"] == "private" {
		vis = "private "
	}
	m.w.line(vis + "import " + m.ref(target, m.scope) + "::*;")
	m.add(e, Mapped, m.v2Name(target), "")
}

// classifier writes a package, classifier or other packaged element.
func (m *migration) classifier(e *xmi.Element) {
	cat, note := m.classify(e)
	switch cat {
	case catNone:
		return
	case catLibrary:
		m.add(e, Skipped, "", "profile or library content")
		return
	case catUnmapped:
		m.unmapped(e, note)
		return
	}
	name := e.Name
	if name == "" {
		if cat == catConnectionDef {
			m.association(e)
			return
		}
		name = m.nameFor(e)
		if note == "" {
			note = "the anonymous " + e.Type + " is named " + name
		}
	}
	verdict := Mapped
	if note != "" {
		verdict = Approximated
	}
	var header strings.Builder
	if e.Attrs["isAbstract"] == "true" {
		header.WriteString("abstract ")
	}
	header.WriteString(cat.keyword())
	header.WriteByte(' ')
	if cat == catRequirementDef {
		if id := requirementID(e); id != "" {
			header.WriteString("<" + writeName(id) + "> ")
		}
	}
	header.WriteString(writeName(name))
	gens, n := m.generals(e, cat)
	if gens != "" {
		header.WriteString(" :> " + gens)
	}
	if n != "" {
		verdict = Approximated
		note = joinNotes(note, n)
	}
	m.add(e, verdict, m.v2Name(e), note)
	switch cat {
	case catConnectionDef:
		m.association(e)
		return
	case catIndividualDef:
		m.w.block(header.String(), func() { m.individualBody(e) })
		return
	case catVerificationDef:
		m.w.block(header.String(), func() { m.verificationBody(e) })
		return
	case catConstraintDef:
		m.w.block(header.String(), func() { m.constraintBody(e) })
		return
	case catRequirementDef:
		m.w.block(header.String(), func() { m.requirementBody(e) })
		return
	case catEnumDef:
		m.w.block(header.String(), func() {
			m.comments(e)
			for _, lit := range e.Owned("ownedLiteral") {
				m.w.line(writeName(lit.Name) + ";")
			}
			m.stereotypeComments(e)
		})
		return
	}
	m.w.block(header.String(), func() {
		m.body(e)
		m.stereotypeComments(e)
	})
}

// generals writes the specializations of a classifier: its generalizations,
// and for a value type the ScalarValues type it derives from.
func (m *migration) generals(e *xmi.Element, cat category) (string, string) {
	var refs []string
	var notes []string
	for _, g := range e.Owned("generalization") {
		target := m.model.Ref(g, "general")
		if target == nil {
			notes = append(notes, "a generalization refers to nothing in the document")
			continue
		}
		if sv := scalarValue(target); sv != "" {
			refs = append(refs, "ScalarValues::"+sv)
			continue
		}
		if target.IsProxy() || isLibrary(target) {
			notes = append(notes, "generalization of library type "+target.Name+" is not written")
			continue
		}
		if tc, _ := m.classify(target); tc != cat {
			notes = append(notes, "generalization of "+qualifiedName(target)+" is not written: it becomes a "+tc.keyword()+", not a "+cat.keyword())
			continue
		}
		refs = append(refs, m.ref(target, m.scope))
	}
	if cat == catIndividualDef {
		if c := m.model.Ref(e, "classifier"); c != nil {
			refs = append(refs, m.ref(c, m.scope))
		}
	}
	return strings.Join(refs, ", "), strings.Join(notes, "; ")
}

func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + "; " + b
}

// requirementID reads the requirement's Id tag, as SysML or MagicDraw spell it.
func requirementID(e *xmi.Element) string {
	for _, s := range e.Stereotypes {
		for _, tag := range []string{"Id", "id", "ID"} {
			if v := s.Tag(tag); v != "" {
				return v
			}
		}
	}
	return ""
}

func requirementText(e *xmi.Element) string {
	for _, s := range e.Stereotypes {
		for _, tag := range []string{"Text", "text"} {
			if v := s.Tag(tag); v != "" {
				return v
			}
		}
	}
	return ""
}

func (m *migration) requirementBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	if text := requirementText(e); text != "" {
		m.w.lines(prefixFirst("doc ", commentLines(commentText(text))))
		for _, c := range m.ownComments(e) {
			m.w.lines(prefixFirst("comment ", commentLines(commentText(c.Attrs["body"]))))
			m.add(c, Mapped, "", "")
		}
	} else {
		m.comments(e)
	}
	for _, c := range e.Children {
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// constraintBody writes a constraint block: its parameters, then its
// anonymous rule as the result expression.
func (m *migration) constraintBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	var result *xmi.Element
	for _, c := range e.Children {
		if c.Role == "ownedRule" && c.Name == "" && result == nil {
			result = c
			continue
		}
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	if result != nil {
		spec := m.model.Ref(result, "specification")
		if spec == nil {
			spec = firstOwned(result, "specification")
		}
		if spec == nil {
			m.unmapped(result, "the constraint has no specification")
		} else if expr, ok, note := m.valueExpr(spec, e); ok {
			m.w.line(expr)
			m.add(result, verdictFor(note), m.v2Name(e), note)
		} else {
			m.unmappedExpr(result, spec, note)
		}
	}
	m.scope = saved
}

func verdictFor(note string) Verdict {
	if note != "" {
		return Approximated
	}
	return Mapped
}

func firstOwned(e *xmi.Element, role string) *xmi.Element {
	if o := e.Owned(role); len(o) > 0 {
		return o[0]
	}
	return nil
}

// individualBody writes an instance specification's slots as redefinitions
// of the classifier's features with the slot values.
func (m *migration) individualBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	for _, slot := range e.Owned("slot") {
		f := m.model.Ref(slot, "definingFeature")
		if f == nil {
			m.unmapped(slot, "the slot's defining feature is not in the document")
			continue
		}
		kw, _, _ := m.featureKeyword(f, catPartDef)
		if kw != "attribute" {
			m.unmapped(slot, "only slots of value properties are written; "+f.Name+" is a "+kw)
			continue
		}
		var vals []string
		var notes []string
		ok := true
		for _, v := range slot.Owned("value") {
			expr, vok, note := m.valueExpr(v, e)
			if !vok {
				ok = false
				notes = append(notes, note)
				break
			}
			vals = append(vals, expr)
			if note != "" {
				notes = append(notes, note)
			}
		}
		if !ok {
			m.unmapped(slot, strings.Join(notes, "; "))
			continue
		}
		value := ""
		switch len(vals) {
		case 0:
		case 1:
			value = " = " + vals[0]
		default:
			value = " = (" + strings.Join(vals, ", ") + ")"
		}
		m.w.line("attribute :>> " + writeName(f.Name) + value + ";")
		m.add(slot, verdictFor(strings.Join(notes, "; ")), m.v2Name(e)+"::"+writeName(f.Name), strings.Join(notes, "; "))
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// verificationBody writes a test case: the requirements it verifies form its
// objective; its behavior is not migrated.
func (m *migration) verificationBody(e *xmi.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	if extras := m.extras[e]; len(extras) > 0 {
		m.w.block("objective", func() {
			for _, extra := range extras {
				extra()
			}
		})
	}
	m.stereotypeComments(e)
	m.scope = saved
}

// association writes an association or association block as a connection def
// with its member ends, or, for an anonymous association whose ends are
// written as the classifiers' properties, nothing.
func (m *migration) association(e *xmi.Element) {
	ends := m.model.Refs(e, "memberEnd")
	if e.Name == "" && e.Type == "Association" {
		m.add(e, Mapped, "", "the anonymous association is written as its member-end properties")
		return
	}
	name := e.Name
	if name == "" {
		name = m.nameFor(e)
	}
	header := "connection def " + writeName(name)
	if gens, _ := m.generals(e, catConnectionDef); gens != "" {
		header += " :> " + gens
	}
	m.w.block(header, func() {
		saved := m.scope
		m.scope = e
		m.comments(e)
		for _, end := range ends {
			t := m.model.Ref(end, "type")
			typ, tnote := m.typeRef(t, e)
			endName := end.Name
			if endName == "" && t != nil {
				endName = lowerFirst(t.Name)
			}
			decl := "end"
			if endName != "" {
				decl += " " + writeName(endName)
			}
			if typ != "" {
				decl += " : " + typ
			}
			decl += m.multiplicity(end) + ";"
			m.w.line(decl)
			if end.Parent == e {
				m.add(end, verdictFor(tnote), m.v2Name(e)+"::"+writeName(endName), tnote)
			}
		}
		for _, c := range e.Children {
			if c.Role != "ownedEnd" {
				m.member(c)
			}
		}
		for _, extra := range m.extras[e] {
			extra()
		}
		m.stereotypeComments(e)
		m.scope = saved
	})
}

// featureKeyword decides the v2 usage keyword of a v1 property from its type
// and aggregation, given the category of its owner.
func (m *migration) featureKeyword(p *xmi.Element, owner category) (keyword, prefix, note string) {
	t := m.model.Ref(p, "type")
	if p.Type == "Port" {
		return "port", "", ""
	}
	if owner == catConstraintDef {
		return "attribute", "in ", ""
	}
	if t == nil {
		return "ref", "", "the untyped property is written as a reference usage"
	}
	if scalarValue(t) != "" {
		return "attribute", "", ""
	}
	tc, _ := m.classify(t)
	switch tc {
	case catAttributeDef, catEnumDef:
		return "attribute", "", ""
	case catConstraintDef:
		return "constraint", "", ""
	case catPortDef:
		if owner == catPortDef {
			return "attribute", "", "a flow property typed by an interface block is written as an attribute"
		}
		return "port", "", "a property typed by an interface block is written as a port"
	case catRequirementDef:
		return "requirement", "", ""
	case catNone, catLibrary:
		return "attribute", "", "typed by library element " + t.Name + " with no known v2 counterpart"
	case catUnmapped:
		return "ref", "", "typed by " + qualifiedName(t) + ", which is not migrated"
	}
	if owner == catPortDef {
		return "item", "", ""
	}
	switch p.Attrs["aggregation"] {
	case "composite":
		return "part", "", ""
	case "shared":
		return "part", "ref ", "shared aggregation is written as a reference part"
	}
	return "part", "ref ", ""
}

// feature writes a property or port of the current scope.
func (m *migration) feature(p *xmi.Element) {
	ownerCat, _ := m.classify(m.scope)
	kw, prefix, note := m.featureKeyword(p, ownerCat)
	t := m.model.Ref(p, "type")
	typ, tnote := m.typeRef(t, m.scope)
	note = joinNotes(note, tnote)

	var b strings.Builder
	switch p.Attrs["visibility"] {
	case "private":
		b.WriteString("private ")
	case "protected":
		b.WriteString("protected ")
	case "package":
		b.WriteString("private ")
		note = joinNotes(note, "package visibility is written as private")
	}
	if p.Attrs["isAbstract"] == "true" {
		b.WriteString("abstract ")
	}
	if p.Attrs["isDerived"] == "true" {
		b.WriteString("derived ")
	}
	if p.Attrs["isReadOnly"] == "true" && kw == "attribute" {
		b.WriteString("readonly ")
	}
	if p.Type == "Port" {
		dir, dnote := portDirection(p)
		b.WriteString(dir)
		note = joinNotes(note, dnote)
	} else if ownerCat == catPortDef {
		if fp := p.Stereotype("FlowProperty"); fp != nil {
			switch fp.Tag("direction") {
			case "in":
				b.WriteString("in ")
			case "out":
				b.WriteString("out ")
			case "inout":
				b.WriteString("inout ")
			}
		}
	}
	b.WriteString(prefix)
	b.WriteString(kw)
	name := m.nameOf(p)
	target := ""
	if name != "" {
		b.WriteString(" " + writeName(name))
		target = m.v2Name(p)
	}

	// A port typed by anything but an interface block carries its type as one
	// directed feature, since a v2 port is typed by a port def alone.
	payload := ""
	if kw == "port" && p.Type == "Port" && typ != "" {
		switch tc, _ := m.classify(t); {
		case scalarValue(t) != "", tc == catAttributeDef, tc == catEnumDef:
			payload = "attribute"
		case tc == catPartDef:
			payload = "item"
		}
	}
	if typ != "" && payload == "" {
		if p.Type == "Port" && p.Attrs["isConjugated"] == "true" {
			typ = "~" + typ
		}
		b.WriteString(" : " + typ)
	}
	b.WriteString(m.multiplicity(p))
	for _, r := range m.model.Refs(p, "redefinedProperty") {
		b.WriteString(" :>> " + m.featureRef(r))
	}
	for _, r := range m.model.Refs(p, "subsettedProperty") {
		b.WriteString(" :> " + m.featureRef(r))
	}

	var bodyLines []string
	if dv := firstOwned(p, "defaultValue"); dv != nil {
		expr, ok, vnote := m.valueExpr(dv, m.scope)
		if ok {
			b.WriteString(" default = " + expr)
			note = joinNotes(note, vnote)
		} else {
			bodyLines = append(bodyLines, commentLines("default value not migrated: "+describeValue(dv)+" — "+vnote)...)
			note = joinNotes(note, "default value not migrated: "+vnote)
		}
	}
	if payload != "" {
		dir, _ := portDirection(p)
		bodyLines = append(bodyLines, dir+payload+" "+writeName(m.nameFor(p))+" : "+typ+";")
		note = joinNotes(note, "a port typed by a "+t.Type+" is written as a port holding one directed "+payload)
	}

	header := b.String()
	m.add(p, verdictFor(note), target, note)
	m.w.block(header, func() {
		saved := m.scope
		m.scope = p
		m.comments(p)
		m.w.lines(bodyLines)
		for _, c := range p.Children {
			if c.Role != "defaultValue" {
				m.member(c)
			}
		}
		for _, extra := range m.extras[p] {
			extra()
		}
		m.stereotypeComments(p)
		m.scope = saved
	})
}

// portDirection writes the direction prefix of a flow port.
func portDirection(p *xmi.Element) (string, string) {
	fp := p.Stereotype("FlowPort")
	if fp == nil {
		return "", ""
	}
	switch fp.Tag("direction") {
	case "in":
		return "in ", ""
	case "out":
		return "out ", ""
	case "inout":
		return "inout ", ""
	}
	return "", ""
}

// written reports whether e becomes a v2 element that can be referred to.
func (m *migration) written(e *xmi.Element) bool {
	if e == nil || e.IsProxy() {
		return false
	}
	switch e.Type {
	case "Property", "Port":
		return m.written(e.Parent)
	case "Association":
		return e.Name != ""
	}
	cat, _ := m.classify(e)
	return cat.keyword() != ""
}

// featureRef writes a reference to a property from a feature that redefines or
// subsets it: the simple name when it is inherited into the current scope.
func (m *migration) featureRef(r *xmi.Element) string {
	if r.Parent != nil && m.inherits(m.scope, r.Parent) && r.Name != "" {
		return writeName(r.Name)
	}
	return m.ref(r, m.scope)
}

// inherits reports whether classifier e specializes general, transitively.
func (m *migration) inherits(e, general *xmi.Element) bool {
	seen := map[*xmi.Element]bool{}
	var walk func(*xmi.Element) bool
	walk = func(c *xmi.Element) bool {
		if c == nil || seen[c] {
			return false
		}
		seen[c] = true
		for _, g := range c.Owned("generalization") {
			t := m.model.Ref(g, "general")
			if t == general || walk(t) {
				return true
			}
		}
		return false
	}
	return walk(e)
}

// typeRef writes the type of a feature: a ScalarValues type, a reference to a
// migrated classifier, or nothing with a note when the type is not migrated.
func (m *migration) typeRef(t *xmi.Element, scope *xmi.Element) (string, string) {
	if t == nil {
		return "", ""
	}
	if sv := scalarValue(t); sv != "" {
		return "ScalarValues::" + sv, ""
	}
	if t.IsProxy() {
		return "", "type " + t.Name + " lives outside the document and is not written"
	}
	cat, _ := m.classify(t)
	switch cat {
	case catLibrary:
		return "", "library type " + qualifiedName(t) + " has no known v2 counterpart and is not written"
	case catUnmapped, catNone:
		return "", "type " + qualifiedName(t) + " is not migrated and is not written"
	}
	return m.ref(t, scope), ""
}

// multiplicity writes a [lower..upper] multiplicity, or nothing for 1..1.
func (m *migration) multiplicity(p *xmi.Element) string {
	lower, upper := "", ""
	if lv := firstOwned(p, "lowerValue"); lv != nil {
		lower = lv.Attrs["value"]
		if lower == "" {
			lower = "0"
		}
	}
	if uv := firstOwned(p, "upperValue"); uv != nil {
		upper = uv.Attrs["value"]
		if upper == "" {
			upper = "1"
		}
	}
	switch {
	case lower == "" && upper == "":
		return ""
	case lower == "":
		if upper == "1" {
			return ""
		}
		return "[" + upper + "]"
	case upper == "":
		if lower == "1" {
			return ""
		}
		return "[" + lower + "..*]"
	case lower == upper:
		if lower == "1" {
			return ""
		}
		return "[" + lower + "]"
	}
	return "[" + lower + ".." + upper + "]"
}

// connector writes a connector: a binding connector as `bind`, another as
// `connect`, and an item flow it realizes as `flow`.
func (m *migration) connector(c *xmi.Element) {
	ends := c.Owned("end")
	if len(ends) != 2 {
		m.unmapped(c, fmt.Sprintf("a connector with %d ends is not migrated", len(ends)))
		return
	}
	paths := make([]string, 2)
	var notes []string
	for i, end := range ends {
		path, note := m.endPath(end)
		if path == "" {
			m.unmapped(c, note)
			return
		}
		paths[i] = path
		if note != "" {
			notes = append(notes, note)
		}
	}
	note := strings.Join(notes, "; ")
	if c.HasStereotype("BindingConnector") {
		m.w.line("bind " + paths[0] + " = " + paths[1] + ";")
		m.add(c, verdictFor(note), "", note)
	} else {
		decl := "connect " + paths[0] + " to " + paths[1] + ";"
		target := ""
		if c.Name != "" {
			decl = "connection " + writeName(c.Name) + " " + decl
			target = m.v2Name(c)
		}
		m.w.line(decl)
		m.add(c, verdictFor(note), target, note)
	}
	for _, f := range m.flows[c] {
		m.itemFlow(f, paths)
	}
}

// endPath writes the feature path a connector end names: the nested property
// path, then the part with port, then the role.
func (m *migration) endPath(end *xmi.Element) (string, string) {
	role := m.model.Ref(end, "role")
	if role == nil {
		return "", "a connector end names no role in the document"
	}
	var segs []*xmi.Element
	if nce := end.Stereotype("NestedConnectorEnd"); nce != nil {
		for _, id := range nce.Tags["propertyPath"] {
			if p := m.model.Lookup(id); p != nil {
				segs = append(segs, p)
			}
		}
	} else if pwp := m.model.Ref(end, "partWithPort"); pwp != nil {
		segs = append(segs, pwp)
	}
	segs = append(segs, role)
	parts := make([]string, len(segs))
	for i, s := range segs {
		parts[i] = writeName(m.nameFor(s))
	}
	if len(segs) == 1 && role.Parent != m.scope && !m.inherits(m.scope, role.Parent) {
		return "", "the connector end's role " + qualifiedName(role) + " is not a feature of " + qualifiedName(m.scope)
	}
	return strings.Join(parts, "."), ""
}

// itemFlow writes an item flow realized by a connector as a flow between the
// flow properties its ends carry for the conveyed classifier.
func (m *migration) itemFlow(f *xmi.Element, paths []string) {
	conveyed := m.model.Refs(f, "conveyed")
	if len(conveyed) == 0 {
		m.unmapped(f, "the item flow conveys nothing")
		return
	}
	src := m.model.Ref(f, "informationSource")
	dst := m.model.Ref(f, "informationTarget")
	if src == nil || dst == nil {
		m.unmapped(f, "the item flow's source or target is not in the document")
		return
	}
	for _, item := range conveyed {
		sp := m.flowProperty(src, item)
		dp := m.flowProperty(dst, item)
		if sp == nil || dp == nil {
			typ, tnote := m.typeRef(item, m.scope)
			if typ == "" {
				m.unmapped(f, joinNotes("the conveyed classifier is not written", tnote))
				continue
			}
			m.w.lines(commentLines("item flow of " + item.Name + " from " + paths[0] + " to " + paths[1] + " not migrated: no flow property of that type on both ends"))
			m.add(f, Unmapped, "", "no flow property typed by "+item.Name+" on both ends of the realizing connector")
			continue
		}
		m.w.line("flow " + paths[0] + "." + writeName(sp.Name) + " to " + paths[1] + "." + writeName(dp.Name) + ";")
		m.add(f, Mapped, "", "")
	}
}

// flowProperty finds the flow property of a port's type carrying item.
func (m *migration) flowProperty(port, item *xmi.Element) *xmi.Element {
	t := m.model.Ref(port, "type")
	if t == nil {
		return nil
	}
	for _, p := range t.Owned("ownedAttribute") {
		if p.HasStereotype("FlowProperty") && m.model.Ref(p, "type") == item && p.Name != "" {
			return p
		}
	}
	return nil
}

// informationFlow writes an item flow with no realizing connector as a flow
// between its source and target when both are features of the scope.
func (m *migration) informationFlow(f *xmi.Element) {
	if len(m.model.Refs(f, "realizingConnector")) > 0 {
		return
	}
	m.unmapped(f, "an information flow with no realizing connector is not migrated")
}

// rule writes a constraint owned by a classifier as a constraint usage.
func (m *migration) rule(r *xmi.Element) {
	spec := firstOwned(r, "specification")
	if spec == nil {
		m.unmapped(r, "the constraint has no specification")
		return
	}
	expr, ok, note := m.valueExpr(spec, m.scope)
	if !ok {
		m.unmappedExpr(r, spec, note)
		return
	}
	decl := "constraint"
	if r.Name != "" {
		decl += " " + writeName(r.Name)
	}
	m.w.line(decl + " { " + expr + " }")
	m.add(r, verdictFor(note), m.v2Name(r), note)
}

// dependencyEnds resolves a dependency's client and supplier, or explains why
// it cannot be written.
func (m *migration) dependencyEnds(d *xmi.Element) (client, supplier *xmi.Element, note string) {
	client = m.model.Ref(d, "client")
	supplier = m.model.Ref(d, "supplier")
	switch {
	case client == nil || supplier == nil:
		return nil, nil, "the dependency's client or supplier is not in the document"
	case client.IsProxy() || supplier.IsProxy():
		return nil, nil, "the dependency reaches outside the document"
	}
	return client, supplier, ""
}

// placeDependency registers, ahead of writing, a Satisfy or Verify in the body
// of the element it is written in, which may precede the dependency itself;
// one that cannot be placed is written where it stands, with the reason.
func (m *migration) placeDependency(d *xmi.Element) {
	if !d.HasStereotype("Satisfy", "Verify") {
		return
	}
	client, supplier, note := m.dependencyEnds(d)
	if note == "" {
		if d.HasStereotype("Satisfy") {
			note = m.satisfy(d, client, supplier)
		} else {
			note = m.verify(d, client, supplier)
		}
	}
	m.unplaced[d] = note
}

// dependency writes a dependency by the SysML stereotype it carries.
func (m *migration) dependency(d *xmi.Element) {
	if d.HasStereotype("Satisfy", "Verify") {
		if note := m.unplaced[d]; note != "" {
			m.unmapped(d, note)
		}
		return
	}
	client, supplier, note := m.dependencyEnds(d)
	if note != "" {
		m.unmapped(d, note)
		return
	}
	if d.HasStereotype("DeriveReqt") {
		m.derive(d, client, supplier)
		return
	}
	for _, end := range []*xmi.Element{client, supplier} {
		if !m.written(end) {
			m.unmapped(d, "its end "+qualifiedName(end)+" is not migrated")
			return
		}
	}
	switch {
	case d.HasStereotype("Refine"):
		m.w.block("dependency "+m.ref(client, m.scope)+" to "+m.ref(supplier, m.scope), func() {
			m.w.line("@ModelingMetadata::Refinement;")
		})
		m.add(d, Mapped, "", "")
	case d.HasStereotype("Allocate"):
		m.w.line("allocate " + m.ref(client, m.scope) + " to " + m.ref(supplier, m.scope) + ";")
		m.add(d, Mapped, "", "")
	case d.HasStereotype("Trace"):
		m.w.line("dependency " + m.ref(client, m.scope) + " to " + m.ref(supplier, m.scope) + "; /* «Trace» */")
		m.add(d, Approximated, "", "a trace is written as a plain dependency")
	case d.HasStereotype("Copy"):
		m.w.line("dependency " + m.ref(client, m.scope) + " to " + m.ref(supplier, m.scope) + "; /* «Copy» */")
		m.add(d, Approximated, "", "a copy is written as a plain dependency; the text is not kept in step")
	default:
		decl := "dependency "
		if d.Name != "" {
			decl += writeName(d.Name) + " from "
		}
		m.w.line(decl + m.ref(client, m.scope) + " to " + m.ref(supplier, m.scope) + ";")
		note := ""
		if len(d.Stereotypes) > 0 {
			note = "«" + d.Stereotypes[0].Name + "» is written as a plain dependency"
		}
		m.add(d, verdictFor(note), "", note)
	}
}

// satisfy places `satisfy requirement` in the body of the satisfying block, or
// of the block owning the satisfying property.
func (m *migration) satisfy(d, client, req *xmi.Element) string {
	if rc, _ := m.classify(req); rc != catRequirementDef {
		return "the supplier " + qualifiedName(req) + " is not a requirement"
	}
	scope, by := m.usageContext(client)
	if scope == nil {
		return "a satisfy whose client is a " + kindOf(client) + " has no v2 form"
	}
	m.extras[scope] = append(m.extras[scope], func() {
		decl := "satisfy requirement : " + m.ref(req, scope)
		if by != "" {
			decl += " by " + by
		}
		m.w.line(decl + ";")
		m.add(d, Mapped, m.v2Name(scope), "")
	})
	return ""
}

// usageContext finds the body a client's satisfy is written in: the
// classifier itself, or the classifier owning a property, satisfied `by` it.
func (m *migration) usageContext(client *xmi.Element) (*xmi.Element, string) {
	if client.Type == "Property" || client.Type == "Port" {
		if client.Parent == nil {
			return nil, ""
		}
		if cat, _ := m.classify(client.Parent); cat == catPartDef || cat == catPortDef || cat == catConnectionDef {
			return client.Parent, writeName(m.nameFor(client))
		}
		return nil, ""
	}
	switch cat, _ := m.classify(client); cat {
	case catPartDef, catPortDef, catConnectionDef, catIndividualDef, catConstraintDef:
		return client, ""
	}
	return nil, ""
}

// verify places `verify requirement` in the objective of the test case.
func (m *migration) verify(d, client, req *xmi.Element) string {
	if rc, _ := m.classify(req); rc != catRequirementDef {
		return "the supplier " + qualifiedName(req) + " is not a requirement"
	}
	if cc, _ := m.classify(client); cc != catVerificationDef {
		return "a verify whose client is not a test case has no v2 form"
	}
	m.extras[client] = append(m.extras[client], func() {
		m.w.line("verify requirement : " + m.ref(req, client) + ";")
		m.add(d, Mapped, m.v2Name(client), "")
	})
	return ""
}

// derive writes a requirement derivation as a connection def specializing the
// library's Derivation, with the original and derived requirements as ends.
func (m *migration) derive(d, derived, original *xmi.Element) {
	dc, _ := m.classify(derived)
	oc, _ := m.classify(original)
	if dc != catRequirementDef || oc != catRequirementDef {
		m.unmapped(d, "both ends of a derive must be requirements")
		return
	}
	name := d.Name
	if name == "" {
		name = "Derive " + derived.Name
		for i := 2; m.nameTaken(m.scope, name); i++ {
			name = fmt.Sprintf("Derive %s %d", derived.Name, i)
		}
		m.names[d] = name
	}
	m.w.block("connection def "+writeName(name)+" :> RequirementDerivation::Derivation", func() {
		m.w.line("end #RequirementDerivation::original original : " + m.ref(original, d) + ";")
		m.w.line("end #RequirementDerivation::derive derived : " + m.ref(derived, d) + ";")
	})
	m.add(d, Mapped, m.v2Name(d), "")
}

// comments writes the comments documenting e: the first as doc, the rest as
// comments, and a comment annotating other elements as `comment about`.
func (m *migration) comments(e *xmi.Element) {
	first := true
	for _, c := range e.Owned("ownedComment") {
		about := m.model.Refs(c, "annotatedElement")
		others := false
		for _, a := range about {
			if a != e {
				others = true
			}
		}
		text := commentText(c.Attrs["body"])
		if text == "" {
			text = commentText(strings.TrimSpace(firstOwnedText(c, "body")))
		}
		if text == "" {
			m.add(c, Skipped, "", "empty comment")
			continue
		}
		if others {
			refs := make([]string, 0, len(about))
			for _, a := range about {
				if !m.written(a) {
					continue
				}
				refs = append(refs, m.ref(a, m.scope))
			}
			if len(refs) == 0 {
				m.w.lines(prefixFirst("comment ", commentLines(text)))
			} else {
				m.w.lines(prefixFirst("comment about "+strings.Join(refs, ", ")+" ", commentLines(text)))
			}
			m.add(c, Mapped, "", "")
			continue
		}
		if first {
			m.w.lines(prefixFirst("doc ", commentLines(text)))
			first = false
		} else {
			m.w.lines(prefixFirst("comment ", commentLines(text)))
		}
		m.add(c, Mapped, "", "")
	}
}

func firstOwnedText(e *xmi.Element, role string) string {
	if o := firstOwned(e, role); o != nil {
		return o.Text
	}
	return ""
}

// ownComments lists the comments of e annotating e alone.
func (m *migration) ownComments(e *xmi.Element) []*xmi.Element {
	var out []*xmi.Element
	for _, c := range e.Owned("ownedComment") {
		own := true
		for _, a := range m.model.Refs(c, "annotatedElement") {
			if a != e {
				own = false
			}
		}
		if own && commentText(c.Attrs["body"]) != "" {
			out = append(out, c)
		}
	}
	return out
}

// comment writes a comment found outside the ownedComment role.
func (m *migration) comment(c *xmi.Element) {
	text := commentText(c.Attrs["body"])
	if text == "" {
		m.add(c, Skipped, "", "empty comment")
		return
	}
	m.w.lines(prefixFirst("comment ", commentLines(text)))
	m.add(c, Mapped, "", "")
}

func prefixFirst(prefix string, lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	out[0] = prefix + out[0]
	return out
}

// classifyingStereotypes are the SysML and MagicDraw stereotypes the mapping
// consumes; any other applied stereotype is kept as a comment.
var classifyingStereotypes = map[string]bool{
	"Block": true, "InterfaceBlock": true, "ConstraintBlock": true, "ValueType": true, "Unit": true,
	"QuantityKind": true, "Requirement": true, "AbstractRequirement": true, "Satisfy": true, "Verify": true,
	"DeriveReqt": true, "Refine": true, "Trace": true, "Copy": true, "Allocate": true, "TestCase": true,
	"FlowPort": true, "FullPort": true, "ProxyPort": true, "FlowProperty": true, "BindingConnector": true,
	"NestedConnectorEnd": true, "ItemFlow": true, "PartProperty": true, "ValueProperty": true,
	"ReferenceProperty": true, "SharedProperty": true, "ConstraintProperty": true, "ConstraintParameter": true,
	"Stakeholder": true, "View": true, "Viewpoint": true,
}

// stereotypeComments keeps the stereotypes the mapping does not consume, and
// the tags of those it does, as a comment in the element's body.
func (m *migration) stereotypeComments(e *xmi.Element) {
	for _, s := range e.Stereotypes {
		if classifyingStereotypes[s.Name] {
			continue
		}
		if isRequirementStereotype(s.Name) {
			m.w.line("/* «" + s.Name + "» */")
			continue
		}
		var tags []string
		for k, vs := range s.Tags {
			tags = append(tags, k+" = "+strings.Join(vs, ", "))
		}
		sort.Strings(tags)
		text := "applied stereotype «" + s.Name + "»"
		if len(tags) > 0 {
			text += ": " + strings.Join(tags, "; ")
		}
		m.w.lines(commentLines(text))
	}
}

func isRequirementStereotype(name string) bool {
	for _, r := range requirementStereotypes {
		if r == name {
			return true
		}
	}
	return false
}

// unmapped records an element with no v2 form and keeps a trace of it as a
// comment where it would have been written.
func (m *migration) unmapped(e *xmi.Element, note string) {
	m.w.lines(commentLines("not migrated: " + kindOf(e) + " " + describe(e) + " — " + note))
	m.add(e, Unmapped, "", note)
}

// unmappedExpr records a constraint whose expression has no v2 form, keeping
// its text.
func (m *migration) unmappedExpr(r, spec *xmi.Element, note string) {
	m.w.lines(commentLines("not migrated: " + kindOf(r) + " " + describe(r) + " " + describeValue(spec) + " — " + note))
	m.add(r, Unmapped, "", note)
}

func describe(e *xmi.Element) string {
	if e.Name != "" {
		return "'" + e.Name + "'"
	}
	return "(" + e.ID + ")"
}

// describeValue shows a value specification's text for a comment.
func describeValue(v *xmi.Element) string {
	if v.Type == "OpaqueExpression" {
		body, lang := opaqueBody(v)
		if lang != "" {
			return "{" + lang + "} " + body
		}
		return body
	}
	if val, ok := v.Attrs["value"]; ok {
		return val
	}
	return "<" + v.Type + ">"
}
