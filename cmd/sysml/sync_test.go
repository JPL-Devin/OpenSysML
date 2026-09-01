package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const syncedModel = `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Vehicle {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
}
`

const renamedModel = `package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Car {
		@IdentityMetadata::ElementId { id = "8f3a41d0"; }
	}
	part def Wheel;
}
`

// syncFixtures writes the renamed model and the repository graph of the
// original, so the diff has a rename and a create to report.
func syncFixtures(t *testing.T, binary string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "original.sysml")
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(original, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(model, []byte(renamedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, binary, original, "-convert", "ttl", "-o", repo)
	return model, repo
}

func TestSyncDiffDryRunThroughCLI(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	out := run(t, binary, model, "-sync-diff", repo)
	for _, want := range []string{
		"sync diff against project proj-1 branch main",
		"update   8f3a41d0",
		"create   P__Wheel",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run report is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "delete") && !strings.Contains(out, "0 delete(s)") {
		t.Errorf("a rename leaked a delete into the report:\n%s", out)
	}
}

func TestSyncDiffReportsUnconfirmedDeletes(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	// Diffing the repository against itself minus Vehicle: sync the original
	// as the local model against a repo graph that also holds Wheel.
	local := filepath.Join(t.TempDir(), "less.sysml")
	if err := os.WriteFile(local, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	wheelRepo := filepath.Join(t.TempDir(), "repo.ttl")
	run(t, binary, model, "-convert", "ttl", "-o", wheelRepo)

	out := run(t, binary, local, "-sync-diff", wheelRepo, "-sync-confirm-deletes")
	if !strings.Contains(out, "delete   P__Wheel") {
		t.Errorf("the repository-only element is not reported as a delete:\n%s", out)
	}
	if strings.Contains(out, "needs explicit confirmation") {
		t.Errorf("a confirmed delete still reads as unconfirmed:\n%s", out)
	}
	_ = repo
}

func TestSyncDiffConflictExitsNonzero(t *testing.T) {
	binary := buildCLI(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.sysml")
	repo := filepath.Join(dir, "repo.ttl")
	if err := os.WriteFile(model, []byte(syncedModel), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.sysml")
	if err := os.WriteFile(other, []byte(`package P {
	@IdentityMetadata::ProjectRef { projectId = "proj-1"; branch = "main"; }
	part def Other;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, binary, other, "-convert", "ttl", "-o", repo)

	cmd := exec.Command(binary, model, "-sync-diff", repo)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 {
		t.Fatalf("a conflicted diff must exit 1, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "conflict 8f3a41d0") {
		t.Errorf("the missing-id conflict is not reported:\n%s", out)
	}
}

func TestSyncAnnotateWritesMintedIDs(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)
	annotated := filepath.Join(t.TempDir(), "annotated.sysml")

	out := run(t, binary, model, "-sync-diff", repo, "-sync-mint-ids", "-sync-annotate", annotated)
	if !strings.Contains(out, "minted id ") {
		t.Errorf("the minted id is not reported:\n%s", out)
	}
	data, err := os.ReadFile(annotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "@IdentityMetadata::ElementId") ||
		!strings.Contains(string(data), "Wheel") {
		t.Errorf("the annotated notation does not declare the minted id:\n%s", data)
	}
}

func TestSyncAnnotateNeedsMinting(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)

	cmd := exec.Command(binary, model, "-sync-diff", repo, "-sync-annotate", "out.sysml")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("-sync-annotate without -sync-mint-ids must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "-sync-mint-ids") {
		t.Errorf("the refusal does not name the missing opt-in:\n%s", out)
	}
}

func TestSyncStatePinsTheBranch(t *testing.T) {
	binary := buildCLI(t)
	model, repo := syncFixtures(t, binary)
	state := filepath.Join(t.TempDir(), "state.sync.json")
	if err := os.WriteFile(state, []byte(`{"projectId":"proj-1","branch":"dev","lastSeenCommit":"c0ffee"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, model, "-sync-diff", repo, "-sync-state", state)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("a state pinning another branch must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "two branches") {
		t.Errorf("the refusal does not explain the branch rule:\n%s", out)
	}
}

func TestSyncFlagsNeedSyncDiff(t *testing.T) {
	binary := buildCLI(t)
	cmd := exec.Command(binary, "-sync-mint-ids")
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("a sync flag without -sync-diff must be refused, got %v:\n%s", err, out)
	}
	if !strings.Contains(string(out), "-sync-diff") {
		t.Errorf("the refusal does not point at -sync-diff:\n%s", out)
	}
}
