// A branch that enters a sibling region of the same composite state re-enters
// that state: the region the branch does not name restarts at its initial state.
package Test {
	state def RegionCrossPseudostate {
		attribute crossed : Integer = 0;

		initial start;
		state running {
			region left {
				initial lstart;
				state lidle;

				then lstart lidle;
				transition lidle to cross if crossed == 0;
			}
			region right {
				initial rstart;
				state ridle;
				state rtarget { entry { crossed = 1; } }

				then rstart ridle;
			}
		}
		choice cross;

		start then running;

		transition cross to rtarget if crossed == 0;
	}
}
