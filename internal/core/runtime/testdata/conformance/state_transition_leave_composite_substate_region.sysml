// A region's active state may be nested below the state the region declares, so
// leaving the composite state for a target outside it must exit each state on the
// way out once: the region's entry is cleared before the state it is active in is
// exited, or exiting the composite state walks the same states a second time.
package Test {
	state def TransitionLeaveCompositeSubstateRegion {
		attribute midExits : Integer = 0;
		attribute wrapperExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute lidleExits : Integer = 0;

		entry; then start;
		state start;
		state running {
			region left {
				entry; then ls;
				state ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }

				succession first ls then lidle;
			}
			region right {
				entry; then rs;
				state rs;
				state wrapper {
					exit { wrapperExits = wrapperExits + 1; }

					state mid {
						exit { midExits = midExits + 1; }

						region inner {
							entry; then is;
							state is;
							state ideep { exit { deepExits = deepExits + 1; } }

							succession first is then ideep;
							transition first ideep if midExits == 0 then outside;
						}
					}
				}

				succession first rs then mid;
			}
		}
		state outside;

		succession first start then running;
	}
}
