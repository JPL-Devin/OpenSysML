// A transition naming a vertex of a sibling orthogonal region is legal (UML
// 2.5.1 §14.2.3.9): the target is a vertex of the same machine, and taking it
// re-enters the composite state, so the region not named restarts.
package Test {
	state def TransitionSiblingRegion {
		attribute crossed : Integer = 0;

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
		}

		start then running;
	}
}
