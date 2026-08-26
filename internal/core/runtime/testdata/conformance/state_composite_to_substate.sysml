// A transition out of a composite state into one of its own substates is also
// external: the composite is exited and re-entered around the effect.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			entry { log = log * 10 + 1; }
			exit { log = log * 10 + 2; }

			state Step1 {
				entry { log = log * 10 + 3; }
				exit { log = log * 10 + 4; }
			}

			state Step2 {
				entry { log = log * 10 + 5; }
			}
		}

		succession first start then Working::Step1;
		transition Working to Working::Step2 when advance do assign log := log * 10 + 9;
	}
}
