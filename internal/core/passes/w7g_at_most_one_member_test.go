package passes

import "testing"

func TestW7GStateReportsOnlyTheExtraSubactions(t *testing.T) {
	const src = `package S {
		action def A;
		state def St {
			entry action e1 : A;
			entry action e2 : A;
			do action d1 : A;
			do action d2 : A;
			exit action x1 : A;
			exit action x2 : A;
		}
	}`
	diags := constraintDiags(t, src)
	for _, c := range []string{"state-entry-action", "state-do-action", "state-exit-action"} {
		got := only(diags, c)
		if len(got) != 1 {
			t.Fatalf("expected one %s diagnostic, got %v", c, got)
		}
		if got[0].Severity != SeverityError {
			t.Fatalf("%s is an error in the reference, got %v", c, got[0].Severity)
		}
	}
}

func TestW7GStateWithOneSubactionOfEachKindIsSilent(t *testing.T) {
	const src = `package S {
		action def A;
		state def St {
			entry action e : A;
			do action d : A;
			exit action x : A;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("one subaction of each kind is valid, got %v", diags)
	}
}

func TestW7GRequirementReportsOnlyTheExtraSubject(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			subject v1 : V;
			subject v2 : V;
		}
	}`
	diags := only(constraintDiags(t, src), "only-one-subject")
	if len(diags) != 1 || diags[0].Message != msgOnlyOneSubject {
		t.Fatalf("expected one subject diagnostic worded as the reference words it, got %v", diags)
	}
}

func TestW7GCaseReportsOnlyTheExtraSubject(t *testing.T) {
	const src = `package C {
		part def V;
		case def Cs {
			subject v1 : V;
			subject v2 : V;
		}
	}`
	if got := len(only(constraintDiags(t, src), "only-one-subject")); got != 1 {
		t.Fatalf("expected one subject diagnostic, got %d", got)
	}
}

func TestW7GSubjectAfterAnotherParameterIsReported(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			in attribute n;
			subject v : V;
		}
	}`
	diags := only(constraintDiags(t, src), "subject-parameter-position")
	if len(diags) != 1 || diags[0].Message != msgSubjectParameterPosition {
		t.Fatalf("expected the subject-position diagnostic, got %v", diags)
	}
}

func TestW7GSubjectAsTheFirstParameterIsSilent(t *testing.T) {
	const src = `package R {
		part def V;
		requirement def Req {
			subject v : V;
			in attribute n;
		}
	}`
	if diags := only(constraintDiags(t, src), "subject-parameter-position"); len(diags) != 0 {
		t.Fatalf("a subject declared first conforms, got %v", diags)
	}
}

// The reference reports the position rule on a requirement whose first parameter
// is not a subject even when it declares no subject at all (matched run).
func TestW7GParameterBeforeAnAbsentSubjectIsReported(t *testing.T) {
	const src = `package R {
		requirement def Req {
			in attribute n;
		}
	}`
	if got := len(only(constraintDiags(t, src), "subject-parameter-position")); got != 1 {
		t.Fatalf("expected the subject-position diagnostic on the declaration, got %d", got)
	}
}

func TestW7GRequirementWithNoParameterIsSilent(t *testing.T) {
	const src = `package R {
		requirement def Req {
			attribute n;
		}
	}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("a requirement with no parameter has an implicit subject, got %v", diags)
	}
}
