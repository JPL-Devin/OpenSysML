package repl

import "testing"

// ownedConstraintFixture declares named require/assume constraints with a body,
// typed by a constraint definition, and redefined by a specialization.
const ownedConstraintFixture = `package Power {
    constraint def Positive { 600.0 > 0.0 }
    constraint def Shortfall { 300.0 >= 450.0 }
    requirement def Margin {
        require constraint enough { 600.0 >= 450.0 }
        require constraint tooLittle { 300.0 >= 450.0 }
        assume constraint supplied : Positive;
        require constraint typed : Shortfall;
    }
    requirement def Tight :> Margin {
        require constraint :>> enough { 600.0 >= 650.0 }
    }
    requirement def Ample {
        require constraint enough { 600.0 >= 450.0 }
        assume constraint supplied : Positive;
    }
    requirement def Fixed :> Margin {
        require constraint :>> tooLittle { 600.0 >= 450.0 }
        require constraint :>> typed { 600.0 >= 450.0 }
    }
    requirement def Kept :> Margin {
        require constraint :>> tooLittle;
        require constraint :>> typed;
    }
}
`

// TestCheckNamedOwnedConstraint checks a named require/assume constraint by
// qualified name like any other constraint usage.
func TestCheckNamedOwnedConstraint(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckConstraint("Power::Margin::enough"), VerdictHolds,
		"✓ Constraint Power::Margin::enough passed")
	wantVerdict(t, s.CheckConstraint("Power::Margin::tooLittle"), VerdictFails,
		"✗ Constraint Power::Margin::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Margin::supplied"), VerdictHolds,
		"✓ Constraint Power::Margin::supplied passed")
	wantVerdict(t, s.CheckConstraint("Power::Margin::typed"), VerdictFails,
		"✗ Constraint Power::Margin::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Tight::enough"), VerdictFails,
		"✗ Constraint Power::Tight::enough failed",
		"Assertion evaluated to false: 600.0 >= 650.0")
	wantVerdict(t, s.CheckConstraint("Power::Margin"), VerdictUnresolved, "not a constraint")
}

// TestCheckRedefinedNamedOwnedConstraint checks that a redefinition stating its
// own body replaces the redefined constraint's, whether written or typed, while
// one stating none keeps it.
func TestCheckRedefinedNamedOwnedConstraint(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckConstraint("Power::Fixed::tooLittle"), VerdictHolds,
		"✓ Constraint Power::Fixed::tooLittle passed")
	wantVerdict(t, s.CheckConstraint("Power::Fixed::typed"), VerdictHolds,
		"✓ Constraint Power::Fixed::typed passed")
	wantVerdict(t, s.CheckConstraint("Power::Kept::tooLittle"), VerdictFails,
		"✗ Constraint Power::Kept::tooLittle failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckConstraint("Power::Kept::typed"), VerdictFails,
		"✗ Constraint Power::Kept::typed failed",
		"Assertion evaluated to false: 300.0 >= 450.0")
}

// TestCheckRequirementWithNamedOwnedConstraints checks that a requirement
// requires what its named constraints state, typing definitions included.
func TestCheckRequirementWithNamedOwnedConstraints(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckRequirement("Power::Ample"), VerdictHolds,
		"✓ Requirement Power::Ample satisfied")
	wantVerdict(t, s.CheckRequirement("Power::Margin"), VerdictFails,
		"✗ Requirement Power::Margin failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
	wantVerdict(t, s.CheckRequirement("Power::Tight"), VerdictFails,
		"✗ Requirement Power::Tight failed")
}

// TestCheckRequirementMasksRedefinedNamedConstraints checks that a requirement
// requires what its redefining constraints state, not the inherited bodies
// they replace — and still the inherited body a bodiless redefinition keeps.
func TestCheckRequirementMasksRedefinedNamedConstraints(t *testing.T) {
	s := loadSource(t, ownedConstraintFixture)

	wantVerdict(t, s.CheckRequirement("Power::Fixed"), VerdictHolds,
		"✓ Requirement Power::Fixed satisfied")
	wantVerdict(t, s.CheckRequirement("Power::Kept"), VerdictFails,
		"✗ Requirement Power::Kept failed",
		"Required condition evaluated to false: 300.0 >= 450.0")
}
