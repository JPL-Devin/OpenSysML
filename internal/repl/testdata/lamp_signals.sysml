// A lamp driven by signals, so %send has accepts to trigger: a bare signal, a
// signal carrying a level, a part exhibiting the machine and one exhibiting none.
package Lamps {
	private import ScalarValues::*;

	attribute def go;
	attribute def Dim {
		attribute level : Integer;
	}
	attribute def Reset;

	state def Lamp {
		attribute brightness : Integer = 0;
		entry; then off;
		state off;
		transition off_on first off accept go then on;
		state on;
		transition on_dim first on
			accept d : Dim
			do assign brightness := d.level
			then dimmed;
		state dimmed;
	}

	part def Bulb {
		exhibit state lamp : Lamp;
	}

	part def Plain {
		attribute watts : Integer = 60;
	}

	part bulb : Bulb;
	part plain : Plain;
}
