action def WorkflowAction {
	in ref x : Real;
	out ref y : Real;
	
	// Control flow nodes
	first start;
	done end;
	then start end;
	
	// General member (tests Phase B2 fallback)
	attribute counter : Integer;
}
