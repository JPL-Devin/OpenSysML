package test {
	// Two guarded successions out of one ordinary action node, written in the
	// member form (`then <source> <target> if <guard>;`): the guards select which
	// one the token takes, without a decision node.
	action route {
		attribute level : Integer = 2;
		attribute low : Integer = 0;
		attribute high : Integer = 0;

		action check assign level := level * 2;
		action alert assign high := 1;
		action idle assign low := 1;

		first check;
		then check alert if level > 10;
		then check idle if level <= 10;
	}
}
