// A guardless succession into a vertex of a sibling orthogonal region exits its
// source only: KerML StatePerformances::StateTransitionPerformance orders
// `guard then transitionLinkSource.exit`, so the enclosing composite state is
// neither exited nor re-entered and the succession cannot re-enable itself.
package Test {
	state def TransitionCrossRegion {
		attribute crossed : Integer = 0;
		attribute reentered : Integer = 0;

		initial start;
		state running {
			entry { reentered = reentered + 1; }

			region left {
				initial lstart;
				state lidle;

				succession first lstart then lidle;
				transition lidle to rtarget;
			}
			region right {
				initial rstart;
				state ridle;
				state rtarget { entry { crossed = crossed + 1; } }

				succession first rstart then ridle;
			}
		}

		succession first start then running;
	}
}
