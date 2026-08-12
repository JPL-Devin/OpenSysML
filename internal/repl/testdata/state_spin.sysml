// A machine that never settles: each state's completion transition leads back
// to the other, so a drain is only ever ended by a budget.
package Debug {
	state Spin {
		attribute n = 0;

		initial init;
		state a {
			do {
				n = n + 1;
			}
		}
		state b {
			do {
				n = n + 1;
			}
		}

		init then a;
		a then b;
		b then a;
	}
}
