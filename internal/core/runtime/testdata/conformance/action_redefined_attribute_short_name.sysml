package test {
	import ScalarValues::*;

	// A short name is no name: `attribute <sx> redefines x = 5;` still answers to
	// the name it redefines, since KerML derives effectiveName from declaredName
	// alone, so its value binds under that name for the body to read.
	action run : Base {
		attribute <sx> redefines x = 5;
		attribute total : Integer = 0;

		first start;

		action bump {
			assign total := total + x;
		}

		done end;

		then start bump;
		then bump end;
	}

	action def Base {
		attribute x : Integer = 1;
	}
}
