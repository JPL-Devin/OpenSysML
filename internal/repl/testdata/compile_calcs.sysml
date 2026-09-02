// Calculations in the scalar subset the native compiler accepts, each exercising
// a rule the generated code must keep: checked Integer arithmetic, Integer
// division answering a Real, mixed-type widening, short-circuit Booleans,
// branches, loops, early returns and recursion.
package Compiled {
	private import ScalarValues::*;

	calc def Fib {
		in n : Integer;
		return : Integer = if n < 2 ? n else Fib(n - 1) + Fib(n - 2);
	}

	calc def SumTo {
		in n : Integer;
		attribute total : Integer = 0;
		attribute i : Integer = 1;
		while i <= n {
			total = total + i;
			i = i + 1;
		}
		return : Integer = total;
	}

	calc def Arith {
		in a : Integer;
		in b : Integer;
		return : Integer = (a + b) * (a - b) + a % b - -a;
	}

	calc def Quot {
		in a : Integer;
		in b : Integer;
		return : Real = a / b;
	}

	calc def Mixed {
		in a : Real;
		in b : Integer;
		return : Real = (a + b) / 2 - b * 0.5 + a ** 2;
	}

	calc def Pow {
		in a : Integer;
		return : Integer = a ** 3;
	}

	calc def RealPow {
		in a : Real;
		in b : Real;
		return : Real = a ** b;
	}

	calc def Neg {
		in a : Integer;
		return : Integer = -a;
	}

	calc def Logic {
		in p : Boolean;
		in q : Boolean;
		in n : Integer;
		// The right operands of `and`/`or`/`implies` divide by n only when reached.
		return : Boolean = (p and 10 / n > 1) or (q implies 10 % n == 0) or not p;
	}

	calc def Compare {
		in a : Real;
		in b : Integer;
		return : Boolean = a < b or a == b or a >= b and a != b;
	}

	calc def Sign {
		in x : Integer;
		attribute r : Integer = 0;
		if x > 0 {
			r = 1;
		} else if x < 0 {
			r = -1;
		}
		return : Integer = r;
	}

	calc def FirstAbove {
		in limit : Integer;
		attribute i : Integer = 0;
		while true {
			i = i + 1;
			if i * i > limit {
				return : Integer = i;
			}
		}
	}

	calc def Collatz {
		in n : Integer;
		attribute x : Real = n;
		attribute steps : Integer = 0;
		while x != 1.0 {
			if x % 2.0 == 0.0 {
				x = x / 2;
			} else {
				x = 3 * x + 1;
			}
			steps = steps + 1;
		}
		return : Integer = steps;
	}

	calc def Hypot {
		in a : Real;
		in b : Real;
		attribute s : Real = a * a + b * b;
		return : Real = s ** 0.5;
	}

	calc def Trailing {
		in a : Real;
		in b : Integer;
		(a + b) / 2
	}

	calc def Named {
		in lhs : Integer;
		in rhs : Integer;
		return : Real = Arith(b = rhs, a = lhs) + Quot(lhs, rhs) * 0;
	}

	calc def Ratio {
		in a : Integer;
		in b : Integer;
		return : Real = Quot(a, b) + Quot(b, a);
	}

	calc def Even {
		in n : Integer;
		return : Boolean = if n == 0 ? true else Odd(n - 1);
	}

	calc def Odd {
		in n : Integer;
		return : Boolean = if n == 0 ? false else Even(n - 1);
	}

	calc def Big {
		in a : Integer;
		return : Real = a * 1.0e300 * 1.0e10;
	}

	// Both operands can fail; the left one must be reported, as the interpreter does.
	calc def Order {
		in a : Integer;
		in b : Integer;
		return : Integer = (a % a) + (b + b);
	}

	calc fib : Fib;

	calc def Specialized :> Fib;

	calc def ViaUsage {
		in n : Integer;
		return : Integer = fib(n) + Specialized(n = n);
	}

	calc def NamedOrder {
		in a : Integer;
		in b : Integer;
		return : Integer = Arith(b = b + b, a = a % a);
	}

	calc def NamedTwice {
		in a : Integer;
		in b : Integer;
		return : Integer = Arith(a = a % a, b = 1, a = b + b, b = a);
	}

	calc def OrderArgs {
		in a : Integer;
		in b : Integer;
		return : Integer = Arith(a % a, b + b);
	}

	calc def Nat {
		in n : Natural;
		return : Natural = n - 1;
	}

	calc def Pos {
		in n : Positive;
		attribute k : Positive = n;
		k = k - 1;
		return : Integer = k;
	}

	calc def One {
		in n : Integer[1];
		attribute m : Integer[1..1] = n;
		return : Integer[1] = m * 2;
	}

	// Names whose naive encodings collide: X::Y against the unrestricted name 'X::Y',
	// a Unicode name against the text of its own escape.
	package X {
		calc def Y { in n : Integer; return : Integer = n + 1; }
	}
	calc def 'X::Y' { in n : Integer; return : Integer = n + 2; }
	calc def 'Größe' { in n : Integer; return : Integer = n + 3; }
	calc def 'Gr_f6__df_e' { in n : Integer; return : Integer = n + 4; }
	calc def Collide {
		in n : Integer;
		return : Integer = X::Y(n) * 1000 + 'X::Y'(n) * 100 + 'Größe'(n) * 10 + 'Gr_f6__df_e'(n);
	}
}

// Calculations outside the subset, each refused for the reason its name states.
package Refused {
	private import ScalarValues::*;

	calc def StringResult {
		in s : String;
		return : String = s;
	}

	calc def Sequence {
		in n : Integer;
		return : Integer = SequenceFunctions::size(1..n);
	}

	calc def Defaulted {
		in n : Integer = 3;
		return : Integer = n;
	}

	calc def OutOnly {
		in n : Integer;
		out r : Integer = n;
	}

	calc def Library {
		in a : Real;
		return : Real = RealFunctions::sqrt(a);
	}

	// Members of its own would redefine what it inherits; that is not compiled.
	calc def Refined :> Compiled::Fib {
		in n : Integer;
	}

	// The interpreter stores the Real quotient in an Integer attribute; the
	// compiler would have to pick a representation, so it declines.
	calc def Narrowed {
		in n : Integer;
		attribute x : Integer = n;
		x = x / 2;
		return : Integer = x;
	}

	calc def Unbound {
		in n : Integer;
		attribute x : Integer;
		return : Integer = x + n;
	}

	calc def DynamicIntPow {
		in a : Integer;
		in b : Integer;
		return : Integer = a ** b;
	}
}
