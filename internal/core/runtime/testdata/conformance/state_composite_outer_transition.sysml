// A transition whose source is a composite state is enabled while any of its
// substates is active: the event is handled by the composite state, not by the
// substate it was offered to first (UML 14.2.3.9.6, SysML 7.19.4).
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			exit { log = log * 10 + 2; }
			state Step1 {
				exit { log = log * 10 + 1; }
			}
			state Step2;

			transition Step1 to Step2 when Next;
		}
		state Done {
			entry { log = log * 10 + 4; }
		}

		succession first start then Step1;
		transition Working to Done when Abort do assign log := log * 10 + 3;
	}
}
