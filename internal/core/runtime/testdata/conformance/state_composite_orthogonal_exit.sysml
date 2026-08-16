// A transition out of an orthogonal composite state exits every active region,
// each innermost-first and in declaration order, before the composite's own exit
// behavior, the transition effect and the target's entry.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			exit { log = log * 10 + 3; }

			region left {
				initial lstart;
				state lwork {
					exit { log = log * 10 + 1; }
				}

				then lstart lwork;
			}

			region right {
				initial rstart;
				state rwork {
					exit { log = log * 10 + 2; }
				}

				then rstart rwork;
			}
		}
		state Done {
			entry { log = log * 10 + 5; }
		}

		start then Working;
		transition Working to Done when Abort do assign log := log * 10 + 4;
	}
}
