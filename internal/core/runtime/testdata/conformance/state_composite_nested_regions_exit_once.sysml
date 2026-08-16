// Leaving a composite state whose region holds another composite with its own
// region exits each level exactly once, innermost first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Outer {
			exit { log = log * 10 + 1; }

			region left {
				initial lstart;
				state Grouping {
					exit { log = log * 10 + 2; }

					region inner {
						initial gstart;
						state g1 {
							exit { log = log * 10 + 3; }
						}

						then gstart g1;
					}
				}

				then lstart Grouping;
			}
			region right {
				initial rstart;
				state Watching {
					exit { log = log * 10 + 4; }
				}

				then rstart Watching;
			}
		}
		state Done;

		start then Outer;
		transition Outer to Done when stop do assign log := log * 10 + 9;
	}
}
