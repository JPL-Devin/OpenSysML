// A cross-region transition into a state nested inside the target region's own
// composite state moves that state's region: the composite state stays active
// and the state it was running exits, so the region set holds one active state
// per region.
package Test {
	state def TransitionCrossRegionNestedTarget {
		attribute crossed : Integer = 0;
		attribute wrapperEntries : Integer = 0;
		attribute innerExits : Integer = 0;

		initial start;
		state running parallel {
			state left {
				initial lstart;
				state lmid;
				state lidle;

				succession first lstart then lmid;
				succession first lmid then lidle;
				transition lidle to rtarget if crossed == 0;
			}
			state right {
				initial rstart;
				state wrapper parallel {
					entry { wrapperEntries = wrapperEntries + 1; }

					state inner {
						initial istart;
						state ridle { exit { innerExits = innerExits + 1; } }
						state rtarget { entry { crossed = 1; } }

						succession first istart then ridle;
					}
				}

				succession first rstart then wrapper;
			}
		}

		succession first start then running;
	}
}
