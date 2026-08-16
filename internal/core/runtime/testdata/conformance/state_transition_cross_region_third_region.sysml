// A cross-region transition leaves the regions holding neither endpoint alone:
// they are concurrent subperformances of the enclosing StatePerformance and
// nothing in KerML's semantic library terminates them.
package Test {
	state def TransitionCrossRegionThirdRegion {
		attribute crossed : Integer = 0;
		attribute thirdEntries : Integer = 0;
		attribute thirdExits : Integer = 0;

		initial start;
		state running {
			region left {
				initial lstart;
				state lidle;

				then lstart lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rstart;
				state ridle;
				state rtarget { entry { crossed = 1; } }

				then rstart ridle;
			}
			region third {
				initial tstart;
				state tidle {
					entry { thirdEntries = thirdEntries + 1; }
					exit { thirdExits = thirdExits + 1; }
				}

				then tstart tidle;
			}
		}

		start then running;
	}
}
