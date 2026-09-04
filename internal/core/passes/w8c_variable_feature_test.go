package passes

import "testing"

// w8cVariableFeatureMessages counts the variability rules' messages for src analyzed under name.
func w8cVariableFeatureMessages(t *testing.T, name, src string) map[string]int {
	t.Helper()
	got := make(map[string]int)
	for _, d := range w8cLibraryDiagnostics(t, name, src) {
		switch d.Message {
		case msgInitialValueNotVariable, msgConstantNotVariable, msgVariableFeatureOwner:
			got[d.Message]++
		}
	}
	return got
}

func TestW8CInitialValueRequiresVariableKerML(t *testing.T) {
	src := `package P {
	class C {
		feature x : C := null;
		var feature v : C := null;
		feature b : C = null;
		feature d : C default := null;
		feature e : C default = null;
		feature y : C;
		var feature w : C;
	}
	class D :> C {
		feature :>> y := null;
		var feature :>> w := null;
	}
	behavior B {
		in var feature p : C := null;
		in feature q : C := null;
	}
	feature top : C := null;
}`
	// KerML: only `var` (or `const`) declares variability; the owner is immaterial.
	got := w8cVariableFeatureMessages(t, "<t>.kerml", src)
	if got[msgInitialValueNotVariable] != 5 {
		t.Errorf("want five %q (x, d, D::y, q, top), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgConstantNotVariable] != 0 {
		t.Errorf("unexpected owner or constant messages in %v", got)
	}
}

func TestW8CInitialValueRequiresVariableSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	item def I {
		var attribute x : Integer := 1;
		attribute y : Integer := 2;
	}
	part def PD {
		attribute z : Integer := 3;
		part sub : PD := null;
		action a : A := null;
	}
	action def A {
		in attribute p : Integer := 0;
	}
	occurrence def O {
		timeslice slice : O := null;
	}
	attribute def AD {
		attribute w : Integer := 4;
		attribute ok : Integer = 5;
	}
	part top : PD := null;
}`
	// A usage of an occurrence type may time-vary, except a portion or a composite action.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgInitialValueNotVariable] != 4 {
		t.Errorf("want four %q (PD::a, O::slice, AD::w, top), got %v", msgInitialValueNotVariable, got)
	}
	if got[msgVariableFeatureOwner] != 0 || got[msgConstantNotVariable] != 0 {
		t.Errorf("unexpected owner or constant messages in %v", got)
	}
}

func TestW8CConstantRequiresVariableSysML(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def A;
	attribute def B {
		constant attribute x : A;
	}
	part def PD {
		constant attribute c : Integer = 1;
		constant part p : PD;
		constant action a : Act;
	}
	action def Act;
	constant attribute top : Integer = 2;
}`
	// A data type's attribute never varies; a part's does, a composite action does not.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgConstantNotVariable] != 3 {
		t.Errorf("want three %q (B::x, PD::a, top), got %v", msgConstantNotVariable, got)
	}
	if got[msgInitialValueNotVariable] != 0 || got[msgVariableFeatureOwner] != 0 {
		t.Errorf("unexpected initial or owner messages in %v", got)
	}
}

func TestW8CConstantIsVariableKerML(t *testing.T) {
	src := `package P {
	class C {
		const feature k : C;
		const feature j : C := null;
	}
}`
	// KerML `const` implies `var`, so a constant feature is never non-variable.
	got := w8cVariableFeatureMessages(t, "<t>.kerml", src)
	if len(got) != 0 {
		t.Errorf("want silence, got %v", got)
	}
}

func TestW8CVariableFeatureRulesDoNotDoubleReport(t *testing.T) {
	src := `package P {
	private import ScalarValues::*;
	attribute def AD {
		var attribute x : Integer := 1;
	}
}`
	// `var` in a data type is an owner error; a SysML usage's variability is derived, not declared.
	got := w8cVariableFeatureMessages(t, "<t>.sysml", src)
	if got[msgVariableFeatureOwner] != 1 || got[msgInitialValueNotVariable] != 1 {
		t.Errorf("want one owner and one initial message, got %v", got)
	}
}
