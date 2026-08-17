// Two timers in orthogonal regions are ordered by the duration their units
// measure, not by the number written: 30 [s] is due before 1 [min].
package test {
	private import SI::*;

	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			region slow {
				initial sstart;
				state sWaiting {
					accept after 1 [min] then sDone;
				}
				state sDone {
					entry { log = log * 10 + 2; }
				}

				transition sstart to sWaiting;
			}

			region fast {
				initial fstart;
				state fWaiting {
					accept after 30 [s] then fDone;
				}
				state fDone {
					entry { log = log * 10 + 1; }
				}

				transition fstart to fWaiting;
			}
		}

		start then Working;
	}
}
