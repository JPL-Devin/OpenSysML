package SatisfyReference {
	requirement def VehicleSpecification;
	requirement vehicleSpecification;
	viewpoint perspective;
	part vehicle;

	part context {
		satisfy vehicleSpecification by vehicle;
		satisfy requirement viewpointConformance by vehicle;
		satisfy requirement conformance : VehicleSpecification by vehicle;
	}

	view def StructureView {
		satisfy perspective;
	}
}
