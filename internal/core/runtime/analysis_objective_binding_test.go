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
		private import SequenceFunctions::*;
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

		action def Weigh { in s : Ship; out m : Real = s.hullMass; }
		requirement def MassCap { subject mass : Real; require constraint { mass < 2000.0 } }
		analysis def ActionStepped {
			subject ship : Ship;
			action weigh : Weigh { in s = ship; }
			objective : MassCap { subject = weigh.m; }
			assert constraint light { weigh.m < 2000.0 }
			return r : Real = weigh.m;
		}
		analysis actionStepped : ActionStepped { subject ship = test::ship; }
		analysis actionSteppedHeavy : ActionStepped { subject ship = heavy; }

		requirement def PairLimit { subject pair : Ship[2]; require constraint { pair->notEmpty() } }
		requirement def FleetLimit { subject fleet : Ship[1..*]; require constraint { fleet->notEmpty() } }
		analysis def OneForPair { subject ship : Ship; objective : PairLimit; return picked : Ship = ship; }
		analysis def TwoForOne { subject ship : Ship; objective : MassLimit; return picked : Ship[2] = (ship, heavy); }
		analysis def TwoForPair { subject ship : Ship; objective : PairLimit; return picked : Ship[2] = (ship, heavy); }
		analysis def TwoForFleet { subject ship : Ship; objective : FleetLimit; return picked : Ship[2] = (ship, heavy); }
		analysis oneForPair : OneForPair { subject ship = test::ship; }
		analysis twoForOne : TwoForOne { subject ship = test::ship; }
		analysis twoForPair : TwoForPair { subject ship = test::ship; }
		analysis twoForFleet : TwoForFleet { subject ship = test::ship; }
		analysis def RedeclaredPair { subject ship : Ship; objective : PairLimit { subject :>> pair; } return picked : Ship[2] = (ship, heavy); }
		analysis def RedeclaredPairOne { subject ship : Ship; objective : PairLimit { subject :>> pair; } return picked : Ship = ship; }
		analysis redeclaredPair : RedeclaredPair { subject ship = test::ship; }
		analysis redeclaredPairOne : RedeclaredPairOne { subject ship = test::ship; }
	}
`

// qualifiedResultModel binds objective subjects and body assertions to a case's
// unnamed result by qualified name (<Case>::result), the OMG pilot example's form.
const qualifiedResultModel = `
	package test {
		private import ScalarValues::*;
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		part heavy : Ship { attribute :>> hullMass = 5000.0; }

		requirement def MassLimit { subject mass : Real; require constraint { mass < 2000.0 } }

		analysis def Qualified {
			subject vessel : Ship;
			objective : MassLimit { subject = Qualified::result; }
			vessel.hullMass
		}
		analysis qualified : Qualified { subject vessel = test::ship; }
		analysis qualifiedHeavy : Qualified { subject vessel = heavy; }

		analysis def Asserting {
			subject vessel : Ship;
			assert constraint capped { Asserting::result < 2000.0 }
			vessel.hullMass
		}
		analysis asserting : Asserting { subject vessel = test::ship; }
		analysis assertingHeavy : Asserting { subject vessel = heavy; }

		analysis def Returned {
			subject vessel : Ship;
			objective : MassLimit { subject = Returned::result; }
			return : Real = vessel.hullMass;
		}
		analysis returnedHeavy : Returned { subject vessel = heavy; }

		analysis def Library {
			subject vessel : Ship;
			objective : MassLimit { subject = Cases::Case::result; }
			vessel.hullMass
		}
		analysis libraryHeavy : Library { subject vessel = heavy; }

		analysis def Mass { subject vessel : Ship; vessel.hullMass }
		analysis def Outer {
			subject vessel : Ship;
			analysis inner : Mass { subject vessel = vessel; }
			objective : MassLimit { subject = inner.result; }
			return total : Real = inner.result * 2.0;
		}
		analysis outer : Outer { subject vessel = test::ship; }
		analysis outerHeavy : Outer { subject vessel = heavy; }

		analysis usageQualified { subject vessel = heavy; objective : MassLimit { subject = usageQualified::result; } vessel.hullMass }
	}
`

// TestObjectiveSubjectBindsTheQualifiedResult pins that <Case>::result names the run's
// unnamed result in an objective binding, a body assertion and a step's inner.result read.
func TestObjectiveSubjectBindsTheQualifiedResult(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, qualifiedResultModel))
	for _, tc := range []struct {
		fqn, kind, output, value string
		status                   VerdictStatus
	}{
		{"test::qualified", "objective", "result", "1000.0", VerdictSatisfied},
		{"test::qualifiedHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::asserting", "assertion", "result", "1000.0", VerdictSatisfied},
		{"test::assertingHeavy", "assertion", "result", "5000.0", VerdictNotSatisfied},
		{"test::returnedHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::libraryHeavy", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::usageQualified", "objective", "result", "5000.0", VerdictNotSatisfied},
		{"test::outer", "objective", "total", "2000.0", VerdictSatisfied},
		{"test::outerHeavy", "objective", "total", "10000.0", VerdictNotSatisfied},
	} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, tc.fqn), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", tc.fqn, err)
		}
		if len(result.Outputs) != 1 || result.Outputs[0].Name != tc.output || FormatValue(result.Outputs[0].Value) != tc.value {
			t.Errorf("%s: outputs = %+v, want %s = %s", tc.fqn, result.Outputs, tc.output, tc.value)
		}
		if len(result.Verdicts) != 1 || result.Verdicts[0].Kind != tc.kind || result.Verdicts[0].Status != tc.status {
			t.Errorf("%s: verdicts = %+v, want the %s %s", tc.fqn, result.Verdicts, tc.kind, tc.status)
		}
		if tc.status == VerdictNotSatisfied && !strings.Contains(result.Verdicts[0].Detail, "< 2000.0") {
			t.Errorf("%s: detail %q does not quote the failed condition", tc.fqn, result.Verdicts[0].Detail)
		}
	}
}

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

// TestObjectiveSubjectBindsAnActionStepOutput pins that an objective's subject and a body
// assertion read a completed action step's output (`weigh.m`) as the case's outputs do.
func TestObjectiveSubjectBindsAnActionStepOutput(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]VerdictStatus{
		"test::actionStepped":      VerdictSatisfied,
		"test::actionSteppedHeavy": VerdictNotSatisfied,
	} {
		result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
		if err != nil {
			t.Fatalf("RunAnalysis(%s): %v", fqn, err)
		}
		if len(result.Verdicts) != 2 {
			t.Fatalf("%s: verdicts = %+v, want the objective's and the assertion's", fqn, result.Verdicts)
		}
		for _, verdict := range result.Verdicts {
			if verdict.Status != want {
				t.Errorf("%s: %s %s: %s (%s), want %s", fqn, verdict.Kind, verdict.Name, verdict.Status, verdict.Detail, want)
			}
		}
	}
}

// TestObjectiveSubjectDefaultHonoursMultiplicity pins that the defaulted result must fit the
// subject's multiplicity as well as its type: one Ship for a Ship[2] subject is undecided,
// and a redeclaration stating no multiplicity (`subject :>> pair;`) keeps the Ship[2].
func TestObjectiveSubjectDefaultHonoursMultiplicity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, objectiveBindingModel))
	for fqn, want := range map[string]string{
		"test::oneForPair":        "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::redeclaredPairOne": "1 value(s) bound to a feature with multiplicity lower bound 2",
		"test::twoForOne":         "2 value(s) bound to a feature with multiplicity upper bound 1",
	} {
		verdict := objectiveVerdict(t, ctx, idx, fqn)
		if verdict.Status != VerdictUndecided {
			t.Fatalf("%s: %s (%s), want undecided", fqn, verdict.Status, verdict.Detail)
		}
		for _, part := range []string{"case's result", "Cases::Case::obj", "multiplicity violation", want} {
			if !strings.Contains(verdict.Detail, part) {
				t.Errorf("%s: detail %q does not say %q", fqn, verdict.Detail, part)
			}
		}
	}
	for _, fqn := range []string{"test::twoForPair", "test::twoForFleet", "test::redeclaredPair"} {
		if verdict := objectiveVerdict(t, ctx, idx, fqn); verdict.Status != VerdictSatisfied {
			t.Errorf("%s: %s (%s), want satisfied", fqn, verdict.Status, verdict.Detail)
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
	for fqn, want := range map[string]error{
		"test::unbound":    ErrTypeMismatch,
		"test::oneForPair": ErrMultiplicityViolation,
		"test::twoForOne":  ErrMultiplicityViolation,
	} {
		sym := oneSymbol(t, idx, fqn)
		run, err := ctx.calcUsageRun(NewEvalContextIn(ctx, nil, nil), sym)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = run.outputValues(ctx); err != nil {
			t.Fatal(err)
		}
		objectives := ctx.ObjectivesOf(sym, nil)
		if len(objectives) != 1 {
			t.Fatalf("%s: %d objectives, want one", fqn, len(objectives))
		}
		_, err = ctx.objectiveBindings(run, objectives[0].Symbol, objectives[0].Name, run.bindingsFrame(ctx))
		if !errors.Is(err, want) {
			t.Errorf("%s: error = %v, want %v", fqn, err, want)
		}
	}
}
