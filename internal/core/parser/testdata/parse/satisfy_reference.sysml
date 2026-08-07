package SatisfyReference {
	requirement vehicleSpecification;
	viewpoint perspective;
	part vehicle;

	part context {
		satisfy vehicleSpecification by vehicle;
		satisfy requirement viewpointConformance by vehicle;
	}

	view def StructureView {
		satisfy perspective;
	}
}
