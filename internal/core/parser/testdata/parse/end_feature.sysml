connection def PressureSeat {
	end [1] part bead : TireBead;
	end [1] mountingRim : TireMountingRim;
}

connection def Anonymous {
	end [1] : TireBead;
	end [1] part named : TireMountingRim;
}

interface def FuelInterface {
	end supplierPort : FuelOutPort;
	end part consumerPort;
}

part wheelAssy {
	connection : PressureSeat connect
		bead references t.bead to
		mountingRim references w.rim;
	interface : FuelInterface connect
		supplierPort ::> tankAssy.fuelTankPort to
		consumerPort ::> eng.engineFuelPort;
}
