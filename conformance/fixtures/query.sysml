package Demo {
	abstract part def Vehicle {
		attribute mass;
	}
	part def Wheel;
	part vehicle : Vehicle {
		part wheels : Wheel[4];
		attribute vin;
	}
	part spare : Wheel;
}
