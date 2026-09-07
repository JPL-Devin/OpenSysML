package An {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real default = 5.0;
		attribute other : Real default = 7.0;
	}

	calc def Sum {
		in a : Real;
		in b : Real;
		return : Real = a + b;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real = 20.0;
		out total : Real = Sum(s.cost, s.other);
		objective affordable {
			require constraint { total <= limit }
		}
	}

	part ship : Ship;
	part barge : Ship {
		attribute :>> cost = 30.0;
	}

	analysis shipCost : CostAnalysis {
		subject s = ship;
	}

	analysis plain {
		out x : Real = 1.0 + 2.0;
	}
}
