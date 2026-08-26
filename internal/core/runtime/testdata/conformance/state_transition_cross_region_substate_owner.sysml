// The state owning the source's region may itself be a substate rather than a
// state of a region: the concurrent region holding the target is then still found
// by walking out to the region its nearest ancestor is declared in, so that region
// exits the state it was running instead of keeping it beside the target.
package Test {
	state def TransitionCrossRegionSubstateOwner {
		attribute lidleExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute midExits : Integer = 0;
		attribute wrapperExits : Integer = 0;
		attribute runningEntries : Integer = 0;

		initial start;
		state running parallel {
			entry { runningEntries = runningEntries + 1; }

			state left {
				initial ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }
				state lstate;

				succession first ls then lidle;
			}
			state right {
				initial rs;
				state wrapper {
					exit { wrapperExits = wrapperExits + 1; }

					state mid parallel {
						exit { midExits = midExits + 1; }

						state inner {
							initial is;
							state ideep { exit { deepExits = deepExits + 1; } }

							succession first is then ideep;
							transition ideep to lstate if lidleExits == 0;
						}
					}
				}

				succession first rs then mid;
			}
		}

		succession first start then running;
	}
}
