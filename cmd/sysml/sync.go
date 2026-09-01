package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
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
		if err := writeAnnotated(local, set, syncAnnotate); err != nil {
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

// writeAnnotated writes the model back as notation with each minted id
// declared by an @ElementId annotation — the explicit opt-in write-back.
func writeAnnotated(local *rdf.Graph, set *reposync.ChangeSet, path string) error {
	mints := set.Mints()
	if len(mints) == 0 {
		return fmt.Errorf("nothing to annotate: no id was minted, so %s would repeat the model unchanged", path)
	}
	turtle := rdf.WriteTurtle(reposync.WriteBack(local, mints))
	notation, err := export.Convert(path, turtle, export.FormatTurtle, export.FormatSysML)
	if err != nil {
		return fmt.Errorf("write minted ids back into notation: %w", err)
	}
	replaced, err := export.WriteFile(path, notation)
	if err != nil {
		return err
	}
	what := ""
	if replaced {
		what = ", replaced the existing file"
	}
	fmt.Fprintf(os.Stderr, "wrote %s with %d minted id(s) annotated (%d bytes%s)\n", path, len(mints), len(notation), what)
	return nil
}
