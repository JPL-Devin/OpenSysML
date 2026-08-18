// A part whose exhibited machine parks on a change condition over the part's own
// feature, plus the operation that makes that condition true.
package Watch {
	part def Sensor {
		attribute tripped = false;

		exhibit state watching {
			entry; then idle;
			state idle;
			state alerted;

			transition idle to alerted accept when tripped;
		}

		action trip {
			first set;
			action set {
				assign tripped := true;
			}
		}
	}
}
