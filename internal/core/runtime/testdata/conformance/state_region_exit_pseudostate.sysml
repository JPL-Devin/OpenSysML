// A branch of a choice reached from inside an orthogonal region that leaves the
// region set exits every region, in declaration order, and the machine ends with
// a single active state.
package Test {
	state def RegionExitPseudostate {
		attribute exits : Integer = 0;

		region left {
			entry; then lstart;
			state lstart;
			state lwork { exit { exits = exits + 1; } }

			succession first lstart then lwork;
			transition first lwork then leave;
		}

		region right {
			entry; then rstart;
			state rstart;
			state rwork { exit { exits = exits + 10; } }

			succession first rstart then rwork;
		}

		choice leave;
		state stopped;

		transition first leave if exits == 0 then stopped;
	}
}
