// A time trigger on a composite state counts from entering it and fires while a
// substate is active: the substate is exited first, then the composite.
package test {
	state Machine {
		attribute log : Integer = 0;

		entry; then start;
		state start;
		state Working {
			exit { log = log * 10 + 2; }

			state Step1 {
				exit { log = log * 10 + 1; }
			}

			accept after 5 then Done;
		}
		state Done {
			entry { log = log * 10 + 3; }
		}

		succession first start then Step1;
	}
}
