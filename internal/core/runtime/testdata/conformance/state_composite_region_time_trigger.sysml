// A time trigger on a composite state inside an orthogonal region is
// region-local: the deeper substate and the composite are exited, and the
// sibling region keeps its own active state.
package test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working parallel {
			state left {
				initial lstart;
				state Grouping {
					exit { log = log * 10 + 2; }

					state Step1 {
						exit { log = log * 10 + 1; }
					}

					accept after 5 then Recovered;
				}
				state Recovered {
					entry { log = log * 10 + 3; }
				}

				transition lstart to Step1;
			}

			state right {
				initial rstart;
				state Watching;

				succession first rstart then Watching;
			}
		}

		succession first start then Working;
	}
}
