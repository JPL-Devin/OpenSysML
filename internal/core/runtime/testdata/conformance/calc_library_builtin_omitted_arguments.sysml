// A built-in's parameter whose multiplicity admits no value may be left out of
// the call, positionally or by name, and binds no value: `size()` counts an
// empty sequence, `sum()` is its identity, an `if` with no else is null, and a
// trailing index defaults as the library declares.
package test {
	private import ScalarValues::*;

	calc def Omitted {
		out sizeOfNothing = SequenceFunctions::size();
		out sumOfNothing = NumericalFunctions::sum();
		out productOfNothing = NumericalFunctions::product();
		out nothingIsEmpty = SequenceFunctions::isEmpty();
		out includesNothing = SequenceFunctions::includes((1, 2));
		out untaken : Integer[0..1] = ControlFunctions::'if'(false);
		out untakenByName : Integer[0..1] = ControlFunctions::'if'(test = false, thenValue = 1);
		out coalescesNothing : Integer[0..1] = ControlFunctions::'??'();
		out coalescesNullAlone : Integer[0..1] = ControlFunctions::'??'(null);
		out toEnd : Integer[*] = SequenceFunctions::subsequence((1, 2, 3), 2);
		out toEndByName : Integer[*] = SequenceFunctions::subsequence(startIndex = 2, seq = (1, 2, 3));
		out withoutOne : Integer[*] = SequenceFunctions::excludingAt((1, 2, 3), 2);
		out withoutOneByName : Integer[*] = SequenceFunctions::excludingAt(startIndex = 2, seq = (1, 2, 3));
	}

	calc c : Omitted;
}
