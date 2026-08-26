package FirstNamingADeclaredNode {
	// `first s1 then s2;` parses as one member carrying both ends: the name the
	// flow starts at and the successor it reaches. Lowering reads the start from
	// the name, so the shape here is what it depends on.
	action def Compact {
		action s1;
		action s2;
		first s1 then s2;
	}

	// The same start written apart from the succession leaving it.
	action def Split {
		action s1;
		action s2;
		first s1;
		succession first s1 then s2;
	}

	// A `first` end naming nothing the body declares is an initial node of its own.
	action def OwnInitialNode {
		action s1;
		first start;
		succession first start then s1;
	}
}
