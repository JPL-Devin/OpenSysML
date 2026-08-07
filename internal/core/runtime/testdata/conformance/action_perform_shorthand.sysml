package test {
	// The bare `perform X;` shorthand: the statement is named after the action
	// it references (KerML Feature::effectiveName), so the succession names it
	// `increment`, while the reference itself must still resolve outward to the
	// action rather than to the statement.
	action outer {
		attribute base : Integer = 7;
		attribute result : Integer = 0;

		first start;

		perform increment;

		done end;

		then start increment;
		then increment end;
	}

	action increment {
		in base : Integer;
		out result : Integer;

		first begin;

		action bump {
			assign result := base + 5;
		}

		done finish;

		then begin bump;
		then bump finish;
	}
}
