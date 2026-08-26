// The mirror of state_composite_region_depth_order: the region declared first is
// the one whose active state is nested deeper, and it still reacts first.
package Test {
	state Machine {
		attribute log : Integer = 0;

		initial start;
		state Working {
			region left {
				initial lstart;
				state deep {
					region inner {
						initial dstart;
						state d1;
						state d2;

						succession first dstart then d1;
						transition d1 to d2 when Tick do assign log := log * 10 + 1;
					}
				}

				succession first lstart then deep;
			}

			region right {
				initial rstart;
				state r1;
				state r2;

				succession first rstart then r1;
				transition r1 to r2 when Tick do assign log := log * 10 + 2;
			}
		}

		succession first start then Working;
	}
}
