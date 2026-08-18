package Plant {
	port def FluidPort;

	part def Pump {
		port outlet : FluidPort;
	}

	part def Tank {
		port inlet : FluidPort;
	}

	item def Water;

	part def Loop {
		part pump : Pump;
		part tank : Tank;
		connection supply : Connection connect pump.outlet to tank.inlet;
		flow of Water from pump to tank;
		// An end outside the exposed set, reported rather than drawn.
		part sensor : Pump;
	}

	connection def Connection;
}

package PlantViews {
	private import Views::*;

	view loopView {
		expose Plant::Loop;
		render asInterconnectionDiagram;
	}
}
