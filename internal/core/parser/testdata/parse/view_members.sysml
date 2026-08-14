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
		frame concern : SafetyConcern;
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
		render rendering : AsTree;
		satisfy SafetyViewpoint;
	}

	// KerML has no `frame` or `render` keyword, so both are legal names: only a
	// name or the notation's kind keyword can follow the member keyword, and
	// anything else declares a feature named after the keyword.
	part def Anchored {
		frame[0..1] : Engineer;
		render :> frame;
	}
}
