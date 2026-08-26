// A transition out of an orthogonal composite state exits every active region,
// each innermost-first and in declaration order, before the composite's own exit
// behavior, the transition effect and the target's entry.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working parallel {
			exit { log = log * 10 + 3; }

			state left {
				initial lstart;
				state lwork {
					exit { log = log * 10 + 1; }
				}

				succession first lstart then lwork;
			}

			state right {
				initial rstart;
				state rwork {
					exit { log = log * 10 + 2; }
				}

				succession first rstart then rwork;
			}
		}
		state Done {
			entry { log = log * 10 + 5; }
		}

		succession first start then Working;
		transition Working to Done when Abort do assign log := log * 10 + 4;
	}
}
