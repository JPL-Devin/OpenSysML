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

		initial start;
		state running {
			region left {
				initial ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }

				then ls lidle;
			}
			region right {
				initial rs;
				state wrapper {
					exit { wrapperExits = wrapperExits + 1; }

					state mid {
						exit { midExits = midExits + 1; }

						region inner {
							initial is;
							state ideep { exit { deepExits = deepExits + 1; } }

							then is ideep;
							transition ideep to outside if midExits == 0;
						}
					}
				}

				then rs mid;
			}
		}
		state outside;

		start then running;
	}
}
