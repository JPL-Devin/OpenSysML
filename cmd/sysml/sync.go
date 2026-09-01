package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/project"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/interop/reposync"
)

// runSyncDiff computes the dry-run change set between the model named on the
// command line and the repository graph -sync-diff names, keyed by effective
// element id. It reports on stdout and never writes to a repository: applying
// is a separate step, and this run only shows what it would do.
func runSyncDiff(files []string) int {
	if len(files) == 0 {
		return fail(errors.New("no model to sync; name the file, as `sysml model.sysml -sync-diff repo.ttl`"))
	}
	if len(files) > 1 {
		return fail(fmt.Errorf("-sync-diff diffs one model; unexpected extra argument %q", files[1]))
	}
	if syncAnnotate != "" && !syncMintIDs {
		return fail(errors.New("-sync-annotate writes minted ids back into the notation, so it needs -sync-mint-ids"))
	}
	model := files[0]

	local, err := loadSyncGraph(model)
	if err != nil {
		return fail(err)
	}
	repository, err := loadSyncGraph(syncDiffWith)
	if err != nil {
		return fail(err)
	}

	opts := reposync.Options{ConfirmDeletes: syncConfirmDeletes, MintIDs: syncMintIDs}
	if syncBase != "" {
		if opts.Base, err = loadSyncGraph(syncBase); err != nil {
			return fail(err)
		}
	}

	scope, err := reposync.GraphScope(local)
	if err != nil {
		return fail(err)
	}
	state, err := loadSyncState(model, scope)
	if err != nil {
		return fail(err)
	}

	set, err := reposync.Diff(local, repository, opts)
	if err != nil {
		return fail(err)
	}
	fmt.Print(set.Text())
	if state != nil && state.LastSeenCommit != "" && opts.Base == nil {
		fmt.Fprintf(os.Stderr, "note: the sync state last saw commit %s; pass its graph with -sync-base to surface repository changes since then as conflicts\n", state.LastSeenCommit)
	}

	if syncAnnotate != "" {
		if err := writeAnnotated(local, model, set, syncAnnotate); err != nil {
			return fail(err)
		}
	}

	if err := set.Appliable(); err != nil {
		fmt.Fprintf(os.Stderr, "note: applying would be refused: %v\n", err)
	}
	if set.Conflicts() > 0 {
		return exitFailed
	}
	return exitHolds
}

// loadSyncGraph reads a graph for the sync: Turtle as-is, notation converted
// through the identity-carrying RDF mapping.
func loadSyncGraph(path string) (*rdf.Graph, error) {
	format, err := export.FormatOfPath(path)
	if err != nil {
		return nil, export.Advise(err, export.ExtensionAdvice)
	}
	name, data, err := project.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if format == export.FormatTurtle {
		return rdf.ParseTurtle(data)
	}
	return export.SysMLToRDF(name, data)
}

// loadSyncState reads the sync state — -sync-state, or <model>.sync.json
// beside the model — and refuses one pinning another project or branch.
func loadSyncState(model string, scope reposync.Scope) (*reposync.State, error) {
	path := syncState
	if path == "" {
		if project.IsStdin(model) {
			return nil, nil
		}
		path = reposync.StatePath(model)
	}
	state, err := reposync.LoadState(path)
	if err != nil || state == nil {
		return nil, err
	}
	if err := state.Check(scope); err != nil {
		return nil, err
	}
	return state, nil
}

// writeAnnotated appends an about-form ElementId annotation per minted id to
// the model's original text, preserving comments — the explicit opt-in write-back.
func writeAnnotated(local *rdf.Graph, model string, set *reposync.ChangeSet, path string) error {
	minted := mintedChanges(set)
	if len(minted) == 0 {
		return fmt.Errorf("nothing to annotate: no id was minted, so %s would repeat the model unchanged", path)
	}
	if project.IsStdin(model) {
		return errors.New("-sync-annotate keeps the model's text, so the model must be a file, not stdin")
	}
	name, data, err := project.ReadFile(model)
	if err != nil {
		return err
	}
	var out strings.Builder
	out.Write(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		out.WriteByte('\n')
	}
	named := namedElements(local)
	annotated := 0
	for _, change := range minted {
		if !addressable(change.QualifiedName, named) {
			fmt.Fprintf(os.Stderr, "note: %s (%s) has no name to write an annotation against; its minted id %s stays in the repository only\n", change.ID, change.Metaclass, change.MintedID)
			continue
		}
		fmt.Fprintf(&out, "metadata : IdentityMetadata::ElementId about %s { id = \"%s\"; }\n",
			lexer.QualifiedNameText(change.QualifiedName), change.MintedID)
		annotated++
	}
	if annotated == 0 {
		return fmt.Errorf("nothing to annotate: no minted element has a name to write an annotation against, so %s would repeat the model unchanged", path)
	}
	notation := []byte(out.String())
	if _, err := export.SysMLToRDF(name, notation); err != nil {
		return fmt.Errorf("the annotated notation does not analyze: %w", err)
	}
	replaced, err := export.WriteFile(path, notation)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s with %d minted id(s) annotated (%d bytes%s)\n", path, annotated, len(notation), what)
	return nil
}

// namedElements indexes the graph's qualified names that carry a declared
// name, so a positional address (an unnamed declaration) is told apart from
// an element genuinely named that way.
func namedElements(g *rdf.Graph) map[string]bool {
	named := make(map[string]bool)
	for _, triple := range g.Triples() {
		if triple.Predicate.Value != rdf.SysML+"qualifiedName" {
			continue
		}
		if _, ok := g.Object(triple.Subject, rdf.SysML+"declaredName"); ok {
			named[triple.Object.Value] = true
		}
	}
	return named
}

// addressable reports whether a qualified name reaches its element by declared
// names alone: every segment path must belong to an element that declares one.
func addressable(fqn string, named map[string]bool) bool {
	if fqn == "" {
		return false
	}
	segments := strings.Split(fqn, "::")
	for i := range segments {
		if !named[strings.Join(segments[:i+1], "::")] {
			return false
		}
	}
	return true
}

// mintedChanges lists the creates that minted an id, in change-set order.
func mintedChanges(set *reposync.ChangeSet) []reposync.Change {
	var minted []reposync.Change
	for _, change := range set.Changes {
		if change.MintedID != "" {
			minted = append(minted, change)
		}
	}
	return minted
}
