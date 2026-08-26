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
			succession first begin then record;
		}

		fork split;
		join sync;
		done;
		action park;

		succession first start then split;
		succession first split then provide;
		succession first split then monitor;
		succession first provide then sync;
		succession first monitor then sync;

		decide check;
		succession first sync then check;
		if speed > 0 then done;
		else park;
		succession first park then done;

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
