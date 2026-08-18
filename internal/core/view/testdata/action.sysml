package Flows {
	private import ScalarValues::*;

	item def Fuel;

	action def Provide {
		in item fuel : Fuel;
		out item torque : Fuel;
	}

	action def Drive {
		attribute speed : Integer = 0;

		first start;

		action provide : Provide;

		// A nested action with a body of its own, whose flow is rendered as a
		// subtree of the node performing it.
		action monitor {
			in item reading : Fuel;
			first begin;
			action record;
			then begin record;
		}

		fork split;
		join sync;
		done finish;
		action park;

		then start split;
		then split provide;
		then split monitor;
		then provide sync;
		then monitor sync;

		decide check;
		then sync check;
		if speed > 0 then finish;
		else park;
		then park finish;

		flow provide.torque to monitor.reading;
	}
}

package FlowViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view driveView : ActionFlowView {
		expose Flows::Drive;
		render asInterconnectionDiagram;
	}
}
