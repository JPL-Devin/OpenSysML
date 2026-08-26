// A cross-region transition leaves the regions holding neither endpoint alone:
// they are concurrent subperformances of the enclosing StatePerformance and
// nothing in KerML's semantic library terminates them.
package Test {
	state def TransitionCrossRegionThirdRegion {
		attribute crossed : Integer = 0;
		attribute thirdEntries : Integer = 0;
		attribute thirdExits : Integer = 0;

		entry; then start;
		state start;
		state running {
			region left {
				entry; then lstart;
				state lstart;
				state lidle;

				succession first lstart then lidle;
				transition first lidle if crossed == 0 then rtarget;
			}
			region right {
				entry; then rstart;
				state rstart;
				state ridle;
				state rtarget { entry { crossed = 1; } }

				succession first rstart then ridle;
			}
			region third {
				entry; then tstart;
				state tstart;
				state tidle {
					entry { thirdEntries = thirdEntries + 1; }
					exit { thirdExits = thirdExits + 1; }
				}

				succession first tstart then tidle;
			}
		}

		succession first start then running;
	}
}
