// A transition naming a vertex of a sibling orthogonal region is legal: the
// target is a vertex of the same machine, and taking it exits the source only —
// KerML StatePerformances::StateTransitionPerformance orders
// `guard then transitionLinkSource.exit` — so the composite state owning the two
// regions is neither exited nor re-entered.
package Test {
	state def TransitionSiblingRegion {
		attribute crossed : Integer = 0;

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
		}

		succession first start then running;
	}
}
