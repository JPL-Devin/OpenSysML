// Both spellings of assignment to a declared calc output in the body.
package Outputs {
	calc def TwoStmt {
		in n : Integer;
		out a : Integer;
		out b : Integer;
		a = n + 1;
		b := n + 2;
	}
}
