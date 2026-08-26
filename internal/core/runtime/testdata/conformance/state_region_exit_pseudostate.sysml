// A branch of a choice reached from inside an orthogonal region that leaves the
// region set exits every region, in declaration order, and the machine ends with
// a single active state. The regions sit inside `working`, since the choice and
// the state it branches to lie outside the region set.
package Test {
	state def RegionExitPseudostate {
		attribute exits : Integer = 0;

		entry; then working;

		state working parallel {
			state left {
				initial lstart;
				state lwork { exit { exits = exits + 1; } }

				succession first lstart then lwork;
				transition lwork to leave;
			}

			state right {
				initial rstart;
				state rwork { exit { exits = exits + 10; } }

				succession first rstart then rwork;
			}
		}

		choice leave;
		state stopped;

		transition leave to stopped if exits == 0;
	}
}
