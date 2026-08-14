package test {
	import ScalarValues::*;

	// The standard flow spelling: a flow with a name of its own and both ends
	// written as feature chains, which moves the value one action node produced at
	// its output to the pin the next node reads.
	action driveTrain {
		first start;

		action generateTorque {
			out engineTorque : Integer;
			assign engineTorque := 21;
		}

		action amplifyTorque {
			in torqueIn : Integer;
			out amplified : Integer;
			assign amplified := torqueIn * 2;
		}

		done end;

		then start generateTorque;
		then generateTorque amplifyTorque;
		then amplifyTorque end;

		flow generateToAmplify from generateTorque.engineTorque to amplifyTorque.torqueIn;
	}
}
