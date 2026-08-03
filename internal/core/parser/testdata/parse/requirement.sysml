requirement def SafetyRequirement {
	doc /* Safety documentation */
	subject vehicle : Vehicle;
	assume condition { vehicle.speed < 100; }
	require constraint { vehicle.fuel > 0; }
}
