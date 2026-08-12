// A performed action's body was written in the scope of the action that declares
// it, so its names resolve there rather than in the scope performing it: `lib`
// reads Lib's `libLevel` (3.0) even though the performing action's own package
// declares a `libLevel` of its own (100.0).
package Lib {
	public import ScalarValues::*;

	attribute libLevel : Real = 3.0;

	action increment {
		out result : Real;

		first begin;

		action bump {
			assign result := libLevel + 1.0;
		}

		done finish;

		then begin bump;
		then bump finish;
	}
}

package test {
	public import ScalarValues::*;

	attribute libLevel : Real = 100.0;

	action outer {
		attribute result : Real = 0.0;

		first start;

		perform action doIt references Lib::increment;

		done end;

		then start doIt;
		then doIt end;
	}
}
