package Demo {
	part def Vehicle {
		attribute mass = 1500.0;

		constraint massPositive {
			assert mass > 0.0;
		}

		constraint massLight {
			assert mass < 100.0;
		}

		requirement lightEnough {
			require constraint { mass < 2000.0 }
		}

		requirement tiny {
			require constraint { mass < 10.0 }
		}
	}

	requirement def MassLimit {
		subject vehicle : Vehicle;
		attribute maxMass;
		require constraint {
			vehicle.mass <= maxMass
		}
	}

	requirement massLimit : MassLimit {
		attribute :>> maxMass = 2000.0;
	}

	requirement massTiny : MassLimit {
		attribute :>> maxMass = 10.0;
	}

	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}

	part analysis {
		assert satisfy massLimit by sedan;
		assert satisfy massTiny by sedan;
	}

	calc add {
		in x;
		in y;
		return x + y;
	}
}
