// A branch that enters a sibling region of the same composite state exits its
// source only: the composite state is not re-entered, and the region the source
// was active in is left without an active state.
package Test {
	state def RegionCrossPseudostate {
		attribute crossed : Integer = 0;

		initial start;
		state running {
			region left {
				initial lstart;
				state lidle;

				succession first lstart then lidle;
				transition lidle to cross if crossed == 0;
			}
			region right {
				initial rstart;
				state ridle;
				state rtarget { entry { crossed = 1; } }

				succession first rstart then ridle;
			}
		}
		choice cross;

		succession first start then running;

		transition cross to rtarget if crossed == 0;
	}
}
