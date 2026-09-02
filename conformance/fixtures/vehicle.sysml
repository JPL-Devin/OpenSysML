package Demo {
	part def Engine {
		attribute power = 300.0;
	}
	part def Vehicle {
		attribute mass default = 1500.0;
		part engine : Engine;
	}
	part sedan : Vehicle {
		attribute :>> mass = 1200.0;
	}
}
