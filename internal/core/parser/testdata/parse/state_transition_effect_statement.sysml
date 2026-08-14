// The `;` that terminates a transition also terminates the statement written as
// its effect (SysML.xtext EffectBehaviorUsage carries no `;` of its own), in the
// standard `first … then` spelling and in the compact `to` one alike.
package TransitionEffects {
	action def Bump;
	attribute def Alert;
	part operator;

	state def S {
		attribute c : ScalarValues::Integer := 0;
		state idle;
		state alerting;

		transition idle to alerting do perform Bump;
		transition alerting to idle do assign c := c + 1;
		transition first idle do perform Bump then alerting;
		transition first alerting do assign c := c + 1 then idle;
		transition idle to alerting do { assign c := c + 1; };
		transition alerting to idle do action nudge : Bump;
		transition idle to alerting do send Alert() to operator;
	}
}
