// Orthogonal regions react to one event in declaration order, whatever their
// nesting depth: the region declared first reacts before a region whose active
// state is nested deeper.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			region left {
				initial lstart;
				state l1;
				state l2;

				then lstart l1;
				transition l1 to l2 when Tick do assign log := log * 10 + 1;
			}

			region right {
				initial rstart;
				state deep {
					region inner {
						initial dstart;
						state d1;
						state d2;

						then dstart d1;
						transition d1 to d2 when Tick do assign log := log * 10 + 2;
					}
				}

				then rstart deep;
			}
		}

		start then Working;
	}
}
