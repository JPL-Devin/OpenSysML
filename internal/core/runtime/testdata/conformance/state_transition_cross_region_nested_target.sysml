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
		state running {
			region left {
				initial lstart;
				state lmid;
				state lidle;

				then lstart lmid;
				then lmid lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rstart;
				state wrapper {
					entry { wrapperEntries = wrapperEntries + 1; }

					region inner {
						initial istart;
						state ridle { exit { innerExits = innerExits + 1; } }
						state rtarget { entry { crossed = 1; } }

						then istart ridle;
					}
				}

				then rstart wrapper;
			}
		}

		start then running;
	}
}
