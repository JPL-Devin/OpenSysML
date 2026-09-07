package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// objectiveBindingModel states one requirement def checked as the objective of
// analysis cases that bind its subject each way SysML v2 allows, or not at all.
const objectiveBindingModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }

		requirement def MassLimit {
			subject s : Ship;
			attribute limit : Real = 2000.0;
			require constraint { s.hullMass < limit }
		}

		analysis def Keyword { subject ship : Ship; objective : MassLimit { subject = ship; } return r : Real = ship.hullMass; }
		analysis def Named { subject ship : Ship; objective : MassLimit { subject s = ship; } return r : Real = ship.hullMass; }
		analysis def Redefined { subject ship : Ship; objective : MassLimit { subject :>> s = ship; } return r : Real = ship.hullMass; }
		analysis def Unbound { subject ship : Ship; objective : MassLimit; return r : Real = ship.hullMass; }
		analysis def Defaulted { subject ship : Ship; objective : MassLimit; return picked : Ship = ship; }
		analysis def Resultless { subject ship : Ship; objective : MassLimit; out m : Real = ship.hullMass; }
		analysis def Inline { subject ship : Ship; objective { require constraint { ship.hullMass < 2000.0 } } return r : Real = ship.hullMass; }
		analysis def Failing { subject ship : Ship; objective : MassLimit { subject = ship.hullMass / 0.0; } return r : Real = ship.hullMass; }

		analysis keyword : Keyword { subject ship = test::ship; }
		analysis keywordHeavy : Keyword { subject ship = heavy; }
		analysis named : Named { subject ship = test::ship; }
		analysis redefined : Redefined { subject ship = test::ship; }
		analysis unbound : Unbound { subject ship = test::ship; }
		analysis defaulted : Defaulted { subject ship = test::ship; }
		analysis defaultedHeavy : Defaulted { subject ship = heavy; }
		analysis resultless : Resultless { subject ship = test::ship; }
		analysis inline : Inline { subject ship = test::ship; }
		analysis inlineHeavy : Inline { subject ship = heavy; }
		analysis failing : Failing { subject ship = test::ship; }

		analysis usageKeyword { subject ship = test::ship; objective : MassLimit { subject = ship; } return r : Real = ship.hullMass; }
		analysis usageRebound : Unbound { subject ship = heavy; objective :>> obj { subject = ship; } }

		analysis def Pick { subject ship : Ship; return picked : Ship = ship; }
		analysis def Stepped {
			subject ship : Ship;
			analysis inner : Pick;
			objective : MassLimit { subject = inner.picked; }
			return r : Real = inner.picked.hullMass;
		}
		analysis stepped : Stepped { subject ship = test::ship; }
		analysis steppedHeavy : Stepped { subject ship = heavy; }
		analysis def SteppedDefault {
			subject ship : Ship;
			analysis inner : Pick;
			objective : MassLimit;
			return picked : Ship = inner.picked;
		}
		analysis steppedDefault : SteppedDefault { subject ship = heavy; }
	}
`

// objectiveVerdict runs the analysis usage fqn and answers its one objective's verdict.
func objectiveVerdict(t *testing.T, ctx *Context, idx *symbols.Index, fqn string) AnalysisVerdict {
	t.Helper()
	result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatalf("RunAnalysis(%s): %v", fqn, err)
	}
	if len(result.Verdicts) != 1 || result.Verdicts[0].Kind != "objective" {
		t.Fatalf("RunAnalysis(%s) reported %d verdict(s), want the objective's: %+v", fqn, len(result.Verdicts), result.Verdicts)
	}
	return result.Verdicts[0]
}

// TestObjectiveSubjectBinding pins that an objective binds its requirement's subject as a
// requirement usage does — by keyword alone, by name or by redefinition — and it decides the verdict.
func TestObjectiveSubjectBinding(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]VerdictStatus{
		"test::keyword":        VerdictSatisfied,
		"test::keywordHeavy":   VerdictNotSatisfied,
		"test::named":          VerdictSatisfied,
		"test::redefined":      VerdictSatisfied,
		"test::inline":         VerdictSatisfied,
		"test::inlineHeavy":    VerdictNotSatisfied,
		"test::usageKeyword":   VerdictSatisfied,
		"test::usageRebound":   VerdictNotSatisfied,
		"test::stepped":        VerdictSatisfied,
		"test::steppedHeavy":   VerdictNotSatisfied,
		"test::defaulted":      VerdictSatisfied,
		"test::defaultedHeavy": VerdictNotSatisfied,
		"test::steppedDefault": VerdictNotSatisfied,
	} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != want {
			t.Errorf("%s: objective %s, want %s (%s)", fqn, verdict.Status, want, verdict.Detail)
		}
		if want == VerdictNotSatisfied && !strings.Contains(verdict.Detail, "hullMass <") {
			t.Errorf("%s: detail %q does not quote the failed condition", fqn, verdict.Detail)
		}
	}
}

// TestObjectiveSubjectDefaultsToTheResult pins the library's default for an unbound objective
// subject (Cases::Case::obj): a result of the wrong type, or none, is undecided for that reason.
func TestObjectiveSubjectDefaultsToTheResult(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))

	mismatch := objectiveVerdict(t, ctx, idx, "test::unbound")
	if mismatch.Status != VerdictUndecided {
		t.Fatalf("a Real result for a Ship subject: %s, want undecided", mismatch.Status)
	}
	for _, want := range []string{"subject s", "case's result", "Cases::Case::obj", "type mismatch", "1000.0 (a Real) is not a Ship"} {
		if !strings.Contains(mismatch.Detail, want) {
			t.Errorf("detail %q does not say %q", mismatch.Detail, want)
		}
	}
	if strings.Contains(mismatch.Detail, "no value for feature") {
		t.Errorf("detail %q reports the symptom, not the mismatch", mismatch.Detail)
	}

	unbound := objectiveVerdict(t, ctx, idx, "test::resultless")
	if unbound.Status != VerdictUndecided {
		t.Fatalf("a case returning no result: %s, want undecided", unbound.Status)
	}
	for _, want := range []string{"objective obj", "s subject is unbound", "return a result"} {
		if !strings.Contains(unbound.Detail, want) {
			t.Errorf("detail %q does not say %q", unbound.Detail, want)
		}
	}
}

// TestObjectiveBindingFailureIsUndecided pins that a subject binding that cannot be evaluated
// leaves the objective undecided, naming the binding, while the case's outputs are still reported.
func TestObjectiveBindingFailureIsUndecided(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	result, err := ctx.RunAnalysis(oneSymbol(t, idx, "test::failing"), AnalysisArgs{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Outputs) != 1 || result.Outputs[0].Name != "r" {
		t.Errorf("outputs = %+v, want r", result.Outputs)
	}
	verdict := result.Verdicts[0]
	if verdict.Status != VerdictUndecided {
		t.Fatalf("verdict %s (%s), want undecided", verdict.Status, verdict.Detail)
	}
	for _, want := range []string{"objective obj", "subject binding evaluation failed", "division by zero"} {
		if !strings.Contains(verdict.Detail, want) {
			t.Errorf("detail %q does not say %q", verdict.Detail, want)
		}
	}
}

// TestObjectiveSubjectBindingOnAnObject pins that an objective of a usage owned by an object
// reads the object's parts (`subject = h;`), as the case's own subject binding does.
func TestObjectiveSubjectBindingOnAnObject(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			part def Ship { attribute hullMass : Real = 1000.0; }
			requirement def MassLimit {
				subject s : Ship;
				require constraint { s.hullMass < 2000.0 }
			}
			part def Holder {
				part h : Ship;
				analysis inner { subject ship = h; objective : MassLimit { subject = h; } return r : Real = ship.hullMass; }
				analysis viaResult { subject ship = h; objective : MassLimit; return : Ship = ship; }
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	holder, err := ctx.Instantiate(oneSymbol(t, idx, "test::Holder"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fqn := range []string{"test::Holder::inner", "test::Holder::viaResult"} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, holder)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", fqn, err)
		}
		if len(result.Verdicts) != 1 || result.Verdicts[0].Status != VerdictSatisfied {
			t.Errorf("%s: verdicts = %+v, want the objective satisfied", fqn, result.Verdicts)
		}
	}
}

// TestObjectiveBindingKeepsErrorIdentity pins that the undecided detail of a
// mismatched default carries the typed error the requirement engine raises.
func TestObjectiveBindingKeepsErrorIdentity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	sym := oneSymbol(t, idx, "test::unbound")
	run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, nil, nil), sym)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = run.outputValues(ctx); err != nil {
		t.Fatal(err)
	}
	objectives := ctx.ObjectivesOf(sym, nil)
	if len(objectives) != 1 {
		t.Fatalf("%d objectives, want one", len(objectives))
	}
	_, err = ctx.objectiveBindings(run, objectives[0].Symbol, objectives[0].Name, run.bindings(ctx))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("error = %v, want ErrTypeMismatch", err)
	}
}
