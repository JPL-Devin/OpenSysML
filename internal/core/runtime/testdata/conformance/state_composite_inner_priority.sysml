// Both the substate and the composite state enclosing it react to Abort: the
// innermost enabled transition wins, so Step1 moves to Step2 and Working is not
// left (UML 14.2.3.9.6 conflict resolution by nesting depth).
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			exit { log = log * 10 + 9; }
			state Step1 {
				exit { log = log * 10 + 1; }
			}
			state Step2 {
				entry { log = log * 10 + 2; }
			}

			transition Step1 to Step2 when Abort;
		}
		state Done {
			entry { log = log * 10 + 9; }
		}

		succession first start then Step1;
		transition Working to Done when Abort;
	}
}
