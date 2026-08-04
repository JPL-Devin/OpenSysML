state def TransitionVariants {
	state s1;
	state s2;
	state s3;
	
	// transition with trigger
	transition s1 to s2 when event1;
	
	// General member (tests Phase B2 fallback)
	attribute stateData : String;
}
