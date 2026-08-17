// A state left before its timer expires withdraws the time event it queued, so
// re-entering it times the whole interval again: the sibling region's clock,
// which the exit does not touch, reaches c1 first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		region work {
			initial wstart;
			state waiting {
				accept after 10 then late;
			}
			state away {
				accept after 4 then waiting;
			}
			state late {
				entry { log = log * 10 + 1; }
			}

			transition wstart to waiting;
			transition waiting to away accept pause;
		}
		region clock {
			initial cstart;
			state c0 {
				accept after 12 then c1;
			}
			state c1 {
				entry { log = log * 10 + 2; }
			}

			transition cstart to c0;
		}
	}
}
