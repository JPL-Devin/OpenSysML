// A transition naming a vertex of a sibling orthogonal region is legal: the
// target is a vertex of the same machine, and taking it exits the source only —
// KerML StatePerformances::StateTransitionPerformance orders
// `guard then transitionLinkSource.exit` — so the composite state owning the two
// regions is neither exited nor re-entered.
package Test {
	state def TransitionSiblingRegion {
		attribute crossed : Integer = 0;

		initial start;
		state running parallel {
			state left {
				initial lstart;
				state lidle;

				succession first lstart then lidle;
				transition lidle to rtarget if crossed == 0;
			}
			state right {
				initial rstart;
				state ridle;
				state rtarget { entry { crossed = 1; } }

				succession first rstart then ridle;
			}
		}

		succession first start then running;
	}
}
