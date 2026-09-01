package reposync

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// Kind classifies one entry of a change set.
type Kind string

const (
	// KindCreate is an element the repository does not have yet.
	KindCreate Kind = "create"
	// KindUpdate is an element both sides have, with differing properties.
	KindUpdate Kind = "update"
	// KindDelete is an element only the repository has. Applying it needs
	// explicit confirmation; the diff reports it either way.
	KindDelete Kind = "delete"
	// KindConflict is a disagreement the sync must not resolve on its own.
	KindConflict Kind = "conflict"
)

// ConflictKind says what a conflict entry disagrees about.
type ConflictKind string

const (
	// ConflictMissingID: the annotation names an id the repository branch no
	// longer has.
	ConflictMissingID ConflictKind = "missing-id"
	// ConflictRepositoryChanged: the repository element changed since the
	// last-seen commit, so writing over it would lose someone else's change.
	ConflictRepositoryChanged ConflictKind = "repository-changed"
)

// PropertyDelta is one property whose values differ between the two sides.
type PropertyDelta struct {
	Property   string
	Local      []string
	Repository []string
}

// Change is one entry of a change set, keyed by the subject's effective id.
type Change struct {
	Kind          Kind
	ID            string // effective id: the subject IRI's local name
	Subject       string // full subject IRI
	QualifiedName string
	Metaclass     string
	Declared      bool   // the id was declared by an @ElementId annotation
	MintedID      string // UUID minted for an unannotated create, when minting is on
	Deltas        []PropertyDelta
	Conflict      ConflictKind
	// RequiresConfirmation marks a delete the options did not confirm.
	RequiresConfirmation bool
}

// ChangeSet is what a diff produced for one project scope.
type ChangeSet struct {
	Scope   Scope
	Changes []Change
}

// Options steer a diff. Deletes and id minting are explicit opt-ins; neither
// ever happens silently.
type Options struct {
	// Base is the repository graph at the last-seen commit. With it, an element
	// the repository changed since then surfaces as a conflict rather than
	// being written over, and a declared id the branch never had is a create
	// rather than a missing-id conflict.
	Base *rdf.Graph
	// ConfirmDeletes confirms repository-side deletes; off, the diff still
	// reports them but Appliable refuses.
	ConfirmDeletes bool
	// MintIDs mints a UUID for each unannotated element being created, so the
	// repository can address it stably.
	MintIDs bool
	// NewID overrides the UUID source, for deterministic tests.
	NewID func() (string, error)
}

// Diff compares the local graph against the repository graph of the same
// project scope and returns the change set keyed by effective element id.
func Diff(local, repository *rdf.Graph, opts Options) (*ChangeSet, error) {
	scope, err := GraphScope(local)
	if err != nil {
		return nil, err
	}
	repoScope, err := GraphScope(repository)
	if err != nil {
		return nil, fmt.Errorf("repository graph: %w", err)
	}
	if !scope.IsZero() && !repoScope.IsZero() {
		if !scope.sameProject(repoScope) {
			return nil, fmt.Errorf("the model is scoped to %s but the repository graph carries %s", scope, repoScope)
		}
		if scope.Branch != "" && repoScope.Branch != "" && scope.Branch != repoScope.Branch {
			return nil, fmt.Errorf("%w: the model names branch %q, the repository graph branch %q", ErrTwoBranches, scope.Branch, repoScope.Branch)
		}
	}

	newID := opts.NewID
	if newID == nil {
		newID = MintUUID
	}

	localView := viewOf(local)
	repoView := viewOf(repository)
	var baseView map[string]*subjectView
	if opts.Base != nil {
		baseView = viewOf(opts.Base)
	}

	set := &ChangeSet{Scope: scope}
	for _, subject := range sortedSubjects(localView, repoView) {
		at, inLocal := localView[subject]
		was, inRepo := repoView[subject]
		switch {
		case inLocal && inRepo:
			deltas := propertyDeltas(at, was)
			if len(deltas) == 0 {
				continue
			}
			change := Change{Kind: KindUpdate, Deltas: deltas}
			if base, seen := baseView[subject]; seen && len(propertyDeltas(was, base)) > 0 {
				change = Change{Kind: KindConflict, Conflict: ConflictRepositoryChanged, Deltas: deltas}
			}
			set.add(change, at)
		case inLocal:
			change := Change{Kind: KindCreate}
			if at.declared && len(repoView) > 0 {
				// A declared id the branch does not have was either dropped
				// there — a conflict — or never pushed. Only the last-seen
				// graph can tell the two apart; absent it, never guess.
				if _, everSeen := baseView[subject]; everSeen || opts.Base == nil {
					change = Change{Kind: KindConflict, Conflict: ConflictMissingID}
				}
			}
			if change.Kind == KindCreate && opts.MintIDs && !at.declared && at.mintable {
				minted, err := newID()
				if err != nil {
					return nil, fmt.Errorf("mint an id for %s: %w", at.name(), err)
				}
				change.MintedID = minted
			}
			set.add(change, at)
		default:
			change := Change{Kind: KindDelete, RequiresConfirmation: !opts.ConfirmDeletes}
			set.add(change, was)
		}
	}
	return set, nil
}

// add fills a change's identity fields from the subject view and appends it.
func (cs *ChangeSet) add(change Change, view *subjectView) {
	change.ID = view.id
	change.Subject = view.subject
	change.QualifiedName = view.qualifiedName
	change.Metaclass = view.metaclass
	change.Declared = view.declared
	cs.Changes = append(cs.Changes, change)
}

// Conflicts counts the entries the sync refuses to resolve on its own.
func (cs *ChangeSet) Conflicts() int { return cs.count(KindConflict) }

// UnconfirmedDeletes counts the deletes the options did not confirm.
func (cs *ChangeSet) UnconfirmedDeletes() int {
	n := 0
	for _, change := range cs.Changes {
		if change.Kind == KindDelete && change.RequiresConfirmation {
			n++
		}
	}
	return n
}

func (cs *ChangeSet) count(kind Kind) int {
	n := 0
	for _, change := range cs.Changes {
		if change.Kind == kind {
			n++
		}
	}
	return n
}

// Appliable reports whether the set can be applied as computed: conflicts and
// unconfirmed deletes refuse an apply until a person decides them.
func (cs *ChangeSet) Appliable() error {
	if n := cs.Conflicts(); n > 0 {
		return fmt.Errorf("%d conflict(s) must be resolved before this change set can be applied", n)
	}
	if n := cs.UnconfirmedDeletes(); n > 0 {
		return fmt.Errorf("%d delete(s) need explicit confirmation before this change set can be applied", n)
	}
	return nil
}

// Mints returns the minted ids of a change set, keyed by the derived id each
// one replaces.
func (cs *ChangeSet) Mints() map[string]string {
	mints := map[string]string{}
	for _, change := range cs.Changes {
		if change.MintedID != "" {
			mints[change.ID] = change.MintedID
		}
	}
	return mints
}

// subjectView is one subject reduced to what the comparison needs: its
// identity and its property values, provenance and identity markers excluded.
type subjectView struct {
	subject       string // full IRI
	id            string // effective id: the IRI's local name
	qualifiedName string
	metaclass     string
	declared      bool
	// mintable marks an element proper: memberships and expression nodes
	// derive their ids and are never minted for.
	mintable bool
	props    map[string][]string
}

// name names the subject for messages: its qualified name when it has one.
func (v *subjectView) name() string {
	if v.qualifiedName != "" {
		return v.qualifiedName
	}
	return v.id
}

// Predicates that carry identity or provenance rather than element content:
// they decide how subjects are keyed and scoped, not what an element says.
func identityPredicate(iri string) bool {
	return iri == predProjectID || iri == predBranch || iri == predOrg ||
		iri == rdf.OpenSysML+"declaredId"
}

// viewOf indexes a graph by subject IRI, reducing each subject to the view the
// comparison works on.
func viewOf(g *rdf.Graph) map[string]*subjectView {
	views := map[string]*subjectView{}
	for _, triple := range g.Triples() {
		if !triple.Subject.IsIRI() {
			continue
		}
		view := views[triple.Subject.Value]
		if view == nil {
			view = &subjectView{
				subject: triple.Subject.Value,
				id:      rdf.LocalName(triple.Subject.Value),
				props:   map[string][]string{},
			}
			views[triple.Subject.Value] = view
		}
		if triple.Predicate.Value == rdf.RDFType {
			view.metaclass = rdf.LocalName(triple.Object.Value)
			continue
		}
		if identityPredicate(triple.Predicate.Value) {
			continue
		}
		name := rdf.LocalName(triple.Predicate.Value)
		view.props[name] = append(view.props[name], triple.Object.String())
	}
	for _, view := range views {
		for _, values := range view.props {
			sort.Strings(values)
		}
		if names := view.props["qualifiedName"]; len(names) > 0 {
			view.qualifiedName = strings.Trim(names[0], `"`)
		}
		view.declared = declaredID(g, view)
		view.mintable = mintable(view)
	}
	return views
}

// declaredID mirrors the RDF reader: an explicit declaredId marker, or an id
// that is not the encoding of the subject's qualified name, was declared by an
// annotation rather than derived.
func declaredID(g *rdf.Graph, view *subjectView) bool {
	if g.BoolValue(rdf.IRI(view.subject), rdf.OpenSysML+"declaredId") {
		return true
	}
	if !view.mintableIRI() || view.qualifiedName == "" {
		return false
	}
	return view.id != rdf.EncodeElementID(view.qualifiedName)
}

// mintableIRI reports whether the subject lives in the element namespace,
// where an element proper's id can be declared or minted.
func (v *subjectView) mintableIRI() bool {
	return strings.HasPrefix(v.subject, rdf.Element)
}

// mintable reports whether the subject is an element proper. Memberships and
// expression nodes derive their ids from their element's, classified by
// rdf:type rather than id spelling, which an adversarial id could fake.
func mintable(view *subjectView) bool {
	if !view.mintableIRI() {
		return false
	}
	return view.metaclass != "OwningMembership" && view.metaclass != "FeatureMembership"
}

// propertyDeltas lists the properties whose value sets differ between two
// views of one subject, the metaclass included as a pseudo-property.
func propertyDeltas(local, repository *subjectView) []PropertyDelta {
	var deltas []PropertyDelta
	if local.metaclass != repository.metaclass {
		deltas = append(deltas, PropertyDelta{
			Property:   "rdf:type",
			Local:      []string{local.metaclass},
			Repository: []string{repository.metaclass},
		})
	}
	names := map[string]bool{}
	for name := range local.props {
		names[name] = true
	}
	for name := range repository.props {
		names[name] = true
	}
	sorted := make([]string, 0, len(names))
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		at, was := local.props[name], repository.props[name]
		if !equalValues(at, was) {
			deltas = append(deltas, PropertyDelta{Property: name, Local: at, Repository: was})
		}
	}
	return deltas
}

func equalValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sortedSubjects returns the union of both views' subject IRIs in a stable
// order, so the change set is deterministic.
func sortedSubjects(local, repository map[string]*subjectView) []string {
	seen := map[string]bool{}
	for subject := range local {
		seen[subject] = true
	}
	for subject := range repository {
		seen[subject] = true
	}
	subjects := make([]string, 0, len(seen))
	for subject := range seen {
		subjects = append(subjects, subject)
	}
	sort.Strings(subjects)
	return subjects
}
