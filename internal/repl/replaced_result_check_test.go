package repl

import (
	"strings"
	"testing"
)

// replacedResultFixture redefines constraints with a body of their own over the
// inherited one, which KerML 8.3.4.8 rejects: only one owned or inherited
// result expression is allowed.
const replacedResultFixture = `package Power {
    constraint def Shortfall { 300.0 >= 450.0 }
    requirement def Margin {
        require constraint tooLittle { 300.0 >= 450.0 }
        require constraint typed : Shortfall;
    }
    requirement def Fixed :> Margin {
        require constraint :>> tooLittle { 600.0 >= 450.0 }
        require constraint :>> typed { 600.0 >= 450.0 }
    }
    calc def Plus { in x : Real; x + 1.0 }
    calc def Twice :> Plus { x + 2.0 }
}
`

// TestCheckReplacedResultExpressionIsRejected checks that a redefinition stating
// a body over an inherited result expression is diagnosed, and that neither
// body is silently chosen when the constraint, its requirement or the calc is
// evaluated.
func TestCheckReplacedResultExpressionIsRejected(t *testing.T) {
	s := NewSession()
	res := s.Submit(replacedResultFixture)
	const want = "Only one (owned or inherited) result expression is allowed"
	var hits int
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, want) {
			hits++
		}
	}
	if hits != 3 {
		t.Fatalf("got %d diagnostics saying %q, want 3: %v", hits, want, res.Diagnostics)
	}

	const refused = "more than one result expression, owned or inherited"
	wantVerdict(t, s.CheckConstraint("Power::Fixed::tooLittle"), VerdictUnresolved, refused,
		"`600.0 >= 450.0` is stated over an inherited result expression")
	wantVerdict(t, s.CheckConstraint("Power::Fixed::typed"), VerdictUnresolved, refused)
	wantVerdict(t, s.CheckRequirement("Power::Fixed"), VerdictUnresolved, refused)
	rejectVerdict(t, s.CheckRequirement("Power::Fixed"), "satisfied", "evaluated to false")

	if lines, err := s.EvalExpr("Power::Twice(1.0)"); err == nil || !strings.Contains(err.Error(), refused) {
		t.Errorf("calc with two result expressions evaluated to %v, %v; want it refused with %q", lines, err, refused)
	}
}
