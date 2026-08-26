// A transition out of a region nested deeper than its target's region leaves the
// source's region set up to the level the two share — the source state and the
// composite state holding its region exit — and the target's region exits the
// state it was running instead of keeping it beside the target.
package Test {
	state def TransitionCrossRegionDeepSource {
		attribute lidleExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute wrapperExits : Integer = 0;
		attribute runningEntries : Integer = 0;

		entry; then start;
		state start;
		state running {
			entry { runningEntries = runningEntries + 1; }

			region left {
				entry; then ls;
				state ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }
				state lstate;

				succession first ls then lidle;
			}
			region right {
				entry; then rs;
				state rs;
				state wrapper {
					exit { wrapperExits = wrapperExits + 1; }

					region inner {
						entry; then is;
						state is;
						state ideep { exit { deepExits = deepExits + 1; } }

						succession first is then ideep;
						transition first ideep if lidleExits == 0 then lstate;
					}
				}

				succession first rs then wrapper;
			}
		}

		succession first start then running;
	}
}
