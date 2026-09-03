// The control functions called by name keep their short-circuit semantics: the
// branch a call does not select is never evaluated, so dividing by zero there
// is harmless, and the optional second operand may be left out when the first
// decides the result.
package test {
	private import ScalarValues::*;

	calc def Controls {
		out selectedThen = ControlFunctions::'if'(true, 1, 1 / 0);
		out selectedElse = ControlFunctions::'if'(false, 1 / 0, 2);
		out omittedElse : Integer[0..1] = ControlFunctions::'if'(false, 1);
		out bodyBranch = ControlFunctions::'if'(true, {1 + 1}, 0);
		out andShortCircuits = ControlFunctions::'and'(false, 1 / 0 == 0);
		out andEvaluates = ControlFunctions::'and'(true, 2 > 1);
		out orShortCircuits = ControlFunctions::'or'(true, 1 / 0 == 0);
		out orEvaluates = ControlFunctions::'or'(false, 2 > 1);
		out impliesShortCircuits = ControlFunctions::'implies'(false, 1 / 0 == 0);
		out impliesEvaluates = ControlFunctions::'implies'(true, false);
		out coalescesNull = ControlFunctions::'??'(null, 5);
		out keepsValue = ControlFunctions::'??'(3, 1 / 0);
		out andDecidedByFirst = ControlFunctions::'and'(false);
		out orDecidedByFirst = ControlFunctions::'or'(true);
		out impliesDecidedByFirst = ControlFunctions::'implies'(false);
		out coalescesAlone = ControlFunctions::'??'(5);
	}

	calc c : Controls;
}
