package ViewMembers {
	rendering def AsTree;
	concern def SafetyConcern;
	part def Engineer;
	part def Person;

	// A viewpoint body frames concerns and names its stakeholders and actors.
	// `frame` takes a reference to an existing concern, or declares one after
	// the `concern` keyword (FramedConcernMember / FramedConcernUsage).
	concern safety : SafetyConcern;
	viewpoint def SafetyViewpoint {
		frame safety;
		frame concern c : SafetyConcern;
		frame concern SafetyConcern;
		stakeholder se : Engineer;
		actor reviewer : Person;
	}

	// A view body renders through a reference to an existing rendering, or
	// declares one after the `rendering` keyword (ViewRenderingMember /
	// ViewRenderingUsage).
	view def Base {
		rendering asTree : AsTree;
	}
	view def Derived :> Base {
		render asTree;
		render rendering r2 : AsTree;
		satisfy SafetyViewpoint;
	}
}
