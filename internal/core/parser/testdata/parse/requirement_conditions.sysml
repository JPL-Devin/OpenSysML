package Conditions {
	requirement def MassLimit {
		attribute mass : Real;
		attribute limit : Real;
		assume mass > 0.0;
		require mass <= limit;
		require constraint {
			mass <= limit
			mass > 0.0
		}
	}

	requirement massOfVehicle : MassLimit {
		subject vehicle : Vehicle[1] :>> MassLimit::vehicle;
		attribute :>> limit = 1500.0;
		assume constraint { limit > 0.0 }
	}

	concern def Weight {
		subject;
		require constraint { 1 < 2 }
	}

	viewpoint def Sizing {
		require 1 < 2;
	}

	satisfy massOfVehicle by vehicle01 {
		require 1 < 2;
	}
}
