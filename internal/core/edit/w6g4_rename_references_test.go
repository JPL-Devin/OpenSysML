package edit

import (
	"strings"
	"testing"
)

// renamed applies one rename to src and returns the edited notation, checking
// that the result is as valid as the original was.
func renamed(t *testing.T, name, src, target, newName string) string {
	t.Helper()
	m := loadContent(t, name, src)
	requireClean(t, m)
	res := applyOne(t, m, Rename(target, newName))
	assertOnlySpanChanged(t, m, res)
	requireClean(t, loadContent(t, name, string(res.Content)))
	return string(res.Content)
}

// refusedRename applies one rename expected to be refused and returns the error.
func refusedRename(t *testing.T, name, src, target, newName string) *Error {
	t.Helper()
	m := loadContent(t, name, src)
	requireClean(t, m)
	res, err := Apply(m, []Operation{Rename(target, newName)})
	if res != nil {
		t.Fatalf("refused rename returned content:\n%s", res.Content)
	}
	return editError(t, err)
}

func TestRenameRewritesQualifiedReferences(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tpart def Old;\n" +
		"\tpart a : P::Old;\n\tpart b : Old;\n}\n"
	got := renamed(t, "qualified.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{"part a : P::Fresh;", "part b : Old;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	// Q's own Old is a different element, so neither its declaration nor the
	// reference to it moves.
	if !strings.Contains(got, "package Q {\n\tpart def Old;") {
		t.Fatalf("an unrelated declaration was rewritten:\n%s", got)
	}
	if !strings.Contains(got, "package P {\n\tpart def Fresh;") {
		t.Fatalf("the declaration was not renamed:\n%s", got)
	}
}

func TestRenameRewritesImportedReferences(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::Old;\n" +
		"\tpart a : Old;\n\tpart b : P::Old;\n}\n"
	got := renamed(t, "imported.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{
		"part def Fresh;", "import P::Fresh;", "part a : Fresh;", "part b : P::Fresh;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Old") {
		t.Fatalf("the old name survives:\n%s", got)
	}
}

// A wildcard import names the namespace, not the member, so renaming a member
// leaves the import alone.
func TestRenameLeavesWildcardImportAlone(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::*;\n" +
		"\tpart a : Old;\n}\n"
	got := renamed(t, "wildcard.sysml", src, "P::Old", "Fresh")

	if !strings.Contains(got, "import P::*;") || !strings.Contains(got, "part a : Fresh;") {
		t.Fatalf("wildcard import rename is wrong:\n%s", got)
	}
}

// An alias is a reference: renaming the element rewrites the alias target, and
// the alias's own name — what references read — does not change.
func TestRenameRewritesAliasTargetNotAliasUses(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\talias A for Old;\n\tpart a : A;\n" +
		"\tpart b : Old;\n}\n"
	got := renamed(t, "alias.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{
		"part def Fresh;", "alias A for Fresh;", "part a : A;", "part b : Fresh;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// Renaming the alias itself rewrites what reads the alias, and not the element
// the alias points at.
func TestRenameAliasRewritesItsUses(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\talias A for Old;\n\tpart a : A;\n}\n"
	got := renamed(t, "alias-rename.sysml", src, "P::A", "B")

	for _, want := range []string{"part def Old;", "alias B for Old;", "part a : B;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// A reference written with the element's short name still resolves after the
// long name is renamed, so it is left as written.
func TestRenameLeavesShortNameReferencesAlone(t *testing.T) {
	const src = "package P {\n\tpart def <O> Old;\n\tpart a : O;\n\tpart b : Old;\n}\n"
	got := renamed(t, "shortname.sysml", src, "P::Old", "Fresh")

	for _, want := range []string{"part def <O> Fresh;", "part a : O;", "part b : Fresh;"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

// The rename is refused where the new name already means something else at a
// reference site: rewriting there would silently rebind that reference, which
// re-analysis cannot catch because the name still resolves.
func TestRenameCapturingANameAtAReferenceIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::*;\n" +
		"\tpart def New;\n\tpart a : Old;\n}\n"
	e := refusedRename(t, "capture.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "Q::New") {
		t.Fatalf("refusal does not name what the reference would read: %s", e.Message)
	}
	if len(e.Referring) != 1 || e.Referring[0] != "Q" {
		t.Fatalf("refusal reports referring %v, want [Q]", e.Referring)
	}
}

// The same refusal for a qualified reference: the new name is already a member
// of the namespace the reference qualifies through.
func TestRenameCapturingAQualifiedSegmentIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\tpart def New;\n}\npackage Q {\n" +
		"\tpart a : P::Old;\n}\n"
	e := refusedRename(t, "capture-qualified.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "P::New") {
		t.Fatalf("refusal does not name P::New: %s", e.Message)
	}
}

// Renaming onto a name declared in a scope the element's references are written
// in is refused even when nothing at the declaration shadows it.
func TestRenameShadowingAtAReferenceIsRefused(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n}\npackage Q {\n\tprivate import P::Old;\n" +
		"\tpart def Inner {\n\t\tattribute New;\n\t\tattribute q : Old;\n\t}\n}\n"
	e := refusedRename(t, "shadow-ref.sysml", src, "P::Old", "New")

	if e.Failure != FailureInvalidName {
		t.Fatalf("failure is %s (%s), want invalid-name", e.Failure, e.Message)
	}
	if !strings.Contains(e.Message, "Q::Inner::New") {
		t.Fatalf("refusal does not name the shadowing feature: %s", e.Message)
	}
}

// Every reference is rewritten in one batch, so a rename with many references
// reports one applied edit per rewritten span and nothing else moves.
func TestRenameReportsEveryRewrittenSpan(t *testing.T) {
	const src = "package P {\n\tpart def Old;\n\tpart a : Old;\n\tpart b : P::Old;\n}\n"
	m := loadContent(t, "spans.sysml", src)
	requireClean(t, m)

	res := applyOne(t, m, Rename("P::Old", "Fresh"))
	if len(res.Applied) != 3 {
		t.Fatalf("applied %d edits, want 3", len(res.Applied))
	}
	for _, a := range res.Applied {
		if a.OldText != "Old" || a.NewText != "Fresh" || a.Target != "P::Old" {
			t.Fatalf("applied edit reports %+v", a)
		}
	}
	assertOnlySpanChanged(t, m, res)
}
