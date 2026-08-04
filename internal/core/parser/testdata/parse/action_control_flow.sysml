action def WorkflowAction {
	in ref x : Real;
	out ref y : Real;
	
	// nested actions
	action start;
	action compute {
		in ref data : Real;
		attribute temp : Real;
	}
	
	// General member (tests Phase B2 fallback)
	attribute counter : Integer;
}
