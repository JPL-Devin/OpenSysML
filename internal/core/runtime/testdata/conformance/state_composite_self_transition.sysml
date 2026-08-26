// A transition from a composite state back to itself is external: it exits the
// active substates innermost-first and the composite, runs the effect, then
// re-enters the composite and its regions' initial states.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			entry { log = log * 10 + 1; }
			exit { log = log * 10 + 2; }

			region left {
				initial lstart;
				state l1 {
					entry { log = log * 10 + 3; }
					exit { log = log * 10 + 4; }
				}

				succession first lstart then l1;
			}
		}

		succession first start then Working;
		transition Working to Working when restart do assign log := log * 10 + 9;
	}
}
