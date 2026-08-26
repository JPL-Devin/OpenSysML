// Leaving a composite state whose region holds another composite with its own
// region exits each level exactly once, innermost first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Outer parallel {
			exit { log = log * 10 + 1; }

			state left {
				initial lstart;
				state Grouping parallel {
					exit { log = log * 10 + 2; }

					state inner {
						initial gstart;
						state g1 {
							exit { log = log * 10 + 3; }
						}

						succession first gstart then g1;
					}
				}

				succession first lstart then Grouping;
			}
			state right {
				initial rstart;
				state Watching {
					exit { log = log * 10 + 4; }
				}

				succession first rstart then Watching;
			}
		}
		state Done;

		succession first start then Outer;
		transition Outer to Done when stop do assign log := log * 10 + 9;
	}
}
