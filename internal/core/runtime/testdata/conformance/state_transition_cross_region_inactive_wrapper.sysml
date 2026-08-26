// A cross-region target nested inside a composite state the target region is not
// yet running: the region records the composite state it enters, not the target
// below it, so leaving the machine's composite state exits the target once.
package Test {
	state def TransitionCrossRegionInactiveWrapper {
		attribute crossed : Integer = 0;
		attribute wrapperEntries : Integer = 0;
		attribute rtargetExits : Integer = 0;
		attribute ridleExits : Integer = 0;

		entry; then start;
		state start;
		state running {
			region left {
				entry; then ls;
				state ls;
				state lidle;

				succession first ls then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			region right {
				entry; then rs;
				state rs;
				state ridle { exit { ridleExits = ridleExits + 1; } }
				state wrapper {
					entry { wrapperEntries = wrapperEntries + 1; }

					region inner {
						entry; then is;
						state is;
						state istart;
						state rtarget {
							entry { crossed = 1; }
							exit { rtargetExits = rtargetExits + 1; }
						}

						succession first is then istart;
					}
				}

				succession first rs then ridle;
			}
		}
		state done;

		succession first start then running;
		transition first rtarget if crossed == 1 then done;
	}
}
