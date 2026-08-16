package runtime

import "testing"

// A transition between two regions of one composite state exits its source only:
// KerML StateTransitionPerformance orders `guard then transitionLinkSource.exit`,
// so the composite state and the regions holding neither endpoint are untouched.
func TestCrossRegionTransitionExitsSourceOnly(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute sourceExits : Integer = 0;
		attribute otherExits : Integer = 0;
		attribute otherEntries : Integer = 0;
		attribute runningExits : Integer = 0;

		initial init;
		state running {
			exit { runningExits = runningExits + 1; }

			region left {
				initial ls;
				state lidle { exit { sourceExits = sourceExits + 1; } }
				then ls lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rs;
				state ridle;
				state rtarget { entry { crossed = crossed + 1; } }
				then rs ridle;
			}
			region other {
				initial os;
				state oidle {
					entry { otherEntries = otherEntries + 1; }
					exit { otherExits = otherExits + 1; }
				}
				then os oidle;
			}
		}

		init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"right": "rtarget", "other": "oidle"})
	if got := countVisits(exec.stateVisits, "running"); got != 1 {
		t.Errorf("running entered %d times, want 1 (the composite state is not re-entered)", got)
	}
	for name, want := range map[string]int64{
		"crossed":      1,
		"sourceExits":  1,
		"otherEntries": 1,
		"otherExits":   0,
		"runningExits": 0,
	} {
		if got := intValue(t, exec.stateData, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// A cross-region transition whose target is nested inside the target region's
// composite state moves that inner region: the composite state stays active and
// the state it was running exits, leaving one active state per region.
func TestCrossRegionTransitionIntoNestedTargetExitsTheAbandonedState(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute innerExits : Integer = 0;
		attribute wrapperEntries : Integer = 0;

		initial init;
		state running {
			region left {
				initial ls;
				state lmid;
				state lidle;
				then ls lmid;
				then lmid lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rs;
				state wrapper {
					entry { wrapperEntries = wrapperEntries + 1; }

					region inner {
						initial is;
						state ridle { exit { innerExits = innerExits + 1; } }
						state rtarget { entry { crossed = crossed + 1; } }
						then is ridle;
					}
				}
				then rs wrapper;
			}
		}

		init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"right": "wrapper", "inner": "rtarget"})
	for _, name := range []string{"wrapper", "running"} {
		if got := countVisits(exec.stateVisits, name); got != 1 {
			t.Errorf("%s entered %d times, want 1 (it is neither exited nor re-entered)", name, got)
		}
	}
	for name, want := range map[string]int64{"crossed": 1, "innerExits": 1, "wrapperEntries": 1} {
		if got := intValue(t, exec.stateData, name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// A region whose cross-region target is nested inside a composite state it is not
// running records that composite state as its active one, so the target is not
// claimed by two regions at once and is exited once when the machine leaves.
func TestCrossRegionTransitionIntoInactiveCompositeRecordsTheEnteredState(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute crossed : Integer = 0;
		attribute rtargetExits : Integer = 0;

		initial init;
		state running {
			region left {
				initial ls;
				state lidle;
				then ls lidle;
				transition lidle to rtarget if crossed == 0;
			}
			region right {
				initial rs;
				state ridle;
				state wrapper {
					region inner {
						initial is;
						state istart;
						state rtarget {
							entry { crossed = crossed + 1; }
							exit { rtargetExits = rtargetExits + 1; }
						}
						then is istart;
					}
				}
				then rs ridle;
			}
		}
		state done;

		init then running;
		transition rtarget to done if crossed == 1;
	}
}`)

	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	if got := intValue(t, exec.stateData, "rtargetExits"); got != 1 {
		t.Errorf("rtargetExits = %d, want 1 (the target is exited once, not through two regions)", got)
	}
}

// A source active in a region nested deeper than its target's region leaves its own
// region set up to the level the two regions share: the source state and the
// composite state holding its region exit, the target region's old state exits, and
// the composite state owning both regions stays active.
func TestCrossRegionTransitionFromDeeperRegionExitsUpToTheSharedLevel(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute lidleExits : Integer = 0;
		attribute deepExits : Integer = 0;
		attribute wrapperExits : Integer = 0;

		initial init;
		state running {
			region left {
				initial ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }
				state lstate;
				then ls lidle;
			}
			region right {
				initial rs;
				state wrapper {
					exit { wrapperExits = wrapperExits + 1; }

					region inner {
						initial is;
						state ideep { exit { deepExits = deepExits + 1; } }
						then is ideep;
						transition ideep to lstate if lidleExits == 0;
					}
				}
				then rs wrapper;
			}
		}

		init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lstate"})
	if got := countVisits(exec.stateVisits, "running"); got != 1 {
		t.Errorf("running entered %d times, want 1 (the composite state owning both regions stays active)", got)
	}
	for _, name := range []string{"lidleExits", "deepExits", "wrapperExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}

// The state owning the source's region may be a substate rather than a state of a
// region: the concurrent region holding the target is still found, so it exits the
// state it was running instead of keeping it beside the target.
func TestCrossRegionTransitionFromARegionOwnedByASubstate(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute lidleExits : Integer = 0;
		attribute midExits : Integer = 0;

		initial init;
		state running {
			region left {
				initial ls;
				state lidle { exit { lidleExits = lidleExits + 1; } }
				state lstate;
				then ls lidle;
			}
			region right {
				initial rs;
				state wrapper {
					state mid {
						exit { midExits = midExits + 1; }

						region inner {
							initial is;
							state ideep;
							then is ideep;
							transition ideep to lstate if lidleExits == 0;
						}
					}
				}
				then rs mid;
			}
		}

		init then running;
	}
}`)

	assertRegionConfig(t, exec, map[string]string{"left": "lstate"})
	for _, name := range []string{"lidleExits", "midExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}

// Exiting a state exits its subperformances, so a transition out of a nested
// non-orthogonal state still exits every state up to the endpoints' least common
// ancestor.
func TestNestedTransitionExitsUpToTheLCA(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute innerExits : Integer = 0;
		attribute outerExits : Integer = 0;

		initial init;
		state outer {
			exit { outerExits = outerExits + 1; }

			state inner { exit { innerExits = innerExits + 1; } }
		}
		state done;

		init then inner;
		transition inner to done;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	for _, name := range []string{"innerExits", "outerExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1 (the nested state and its parent are both exited)", name, got)
		}
	}
}

// A transition out of a region whose target lies outside the composite state
// still leaves the whole region set, sibling regions included.
func TestTransitionOutOfCompositeStateExitsEveryRegion(t *testing.T) {
	exec := runStateMachine(t, "Machine", `package P {
	state Machine {
		attribute leftExits : Integer = 0;
		attribute rightExits : Integer = 0;
		attribute runningExits : Integer = 0;

		initial init;
		state running {
			exit { runningExits = runningExits + 1; }

			region left {
				initial ls;
				state lidle { exit { leftExits = leftExits + 1; } }
				then ls lidle;
				transition lidle to done;
			}
			region right {
				initial rs;
				state ridle { exit { rightExits = rightExits + 1; } }
				then rs ridle;
			}
		}
		state done;

		init then running;
	}
}`)

	if len(exec.activeConfig.regionStates) != 0 {
		t.Errorf("regions still active: %v", regionConfig(exec))
	}
	if got := exec.getCurrentState(); got == nil || got.Name != "done" {
		t.Fatalf("current state = %v, want done", got)
	}
	for _, name := range []string{"leftExits", "rightExits", "runningExits"} {
		if got := intValue(t, exec.stateData, name); got != 1 {
			t.Errorf("%s = %d, want 1", name, got)
		}
	}
}
