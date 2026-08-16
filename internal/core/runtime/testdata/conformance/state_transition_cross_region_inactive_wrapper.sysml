// A cross-region target nested inside a composite state the target region is not
// yet running: the region records the composite state it enters, not the target
// below it, so leaving the machine's composite state exits the target once.
package Test {
	state def TransitionCrossRegionInactiveWrapper {
		attribute crossed : Integer = 0;
		attribute wrapperEntries : Integer = 0;
		attribute rtargetExits : Integer = 0;
		attribute ridleExits : Integer = 0;

		initial start;
		state running {
			region left {
				initial ls;
				state lidle;

				then ls lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rs;
				state ridle { exit { ridleExits = ridleExits + 1; } }
				state wrapper {
					entry { wrapperEntries = wrapperEntries + 1; }

					region inner {
						initial is;
						state istart;
						state rtarget {
							entry { crossed = 1; }
							exit { rtargetExits = rtargetExits + 1; }
						}

						then is istart;
					}
				}

				then rs ridle;
			}
		}
		state done;

		start then running;
		transition rtarget to done if crossed == 1;
	}
}
