// Analysis cases stating objectives: what each one improves comes from the
// trade-study definition it is typed by, and what is feasible from the conditions
// the case requires or assumes together with an objective's own.
package test {
	public import SI::*;
	public import ISQ::*;
	private import ScalarValues::*;
	private import TradeStudies::*;

	// A quantity-valued objective: the least mass the budget permits, in the base
	// units the conditions express masses in.
	analysis def MassBudget {
		attribute mass : ISQ::MassValue;
		require constraint {
			mass >= 10 [kg]
		}
		require constraint {
			mass <= 90 [kg]
		}
		objective lightest : MinimizeObjective {
			attribute :>> best = mass;
		}
	}

	// An integer objective, with an assumption bounding it from above and the
	// objective's own condition bounding it from below.
	analysis def CrewSizing {
		attribute crew : Integer;
		assume constraint {
			crew <= 7
		}
		objective largestCrew : MaximizeObjective {
			attribute :>> best = crew;
			require constraint {
				crew >= 2
			}
		}
	}

	// A real objective whose feasible values approach a bound no assignment
	// attains, since the condition is a strict inequality.
	analysis def OpenMargin {
		attribute margin : Real;
		require constraint {
			margin >= 0.0
		}
		require constraint {
			margin < 10.5
		}
		objective widest : MaximizeObjective {
			attribute :>> best = margin;
		}
	}

	// An unbounded objective: nothing bounds the load from above.
	analysis def UnboundedLoad {
		attribute load : Real;
		require constraint {
			load >= 1.0
		}
		objective heaviest : MaximizeObjective {
			attribute :>> best = load;
		}
	}

	// An infeasible case: no assignment satisfies its conditions, so there is
	// nothing to optimize over.
	analysis def ImpossibleFit {
		attribute width : Integer;
		require constraint {
			width >= 10
		}
		require constraint {
			width <= 4
		}
		objective narrowest : MinimizeObjective {
			attribute :>> best = width;
		}
	}

	// Two objectives, improved lexicographically in the order declared: the least
	// cost first, and among the assignments achieving it the greatest margin.
	analysis def CostThenMargin {
		attribute cost : Integer;
		attribute margin : Integer;
		require constraint {
			cost >= 3
		}
		require constraint {
			cost <= 9
		}
		require constraint {
			margin >= 0
		}
		require constraint {
			margin <= cost * 2
		}
		objective cheapest : MinimizeObjective {
			attribute :>> best = cost;
		}
		objective widestMargin : MaximizeObjective {
			attribute :>> best = margin;
		}
	}

	// An objective over a variant selection: which variant is chosen decides the
	// value improved, so the optimum names a configuration.
	part def Wheel {
		attribute rim : Rim;
	}

	attribute def Rim;

	analysis def WheelChoice {
		part wheel : Wheel {
			variation attribute :>> rim {
				variant attribute steel;
				variant attribute carbon;
			}
		}
		require constraint {
			wheel::rim == wheel::rim::steel or wheel::rim == wheel::rim::carbon
		}
		objective lightestRim : MinimizeObjective {
			attribute :>> best = if wheel::rim == wheel::rim::carbon ? 4 else 9;
		}
	}

	// A case whose condition divides by a computed divisor, which is asserted
	// non-zero, while the objective itself stays linear.
	analysis def GuardedRatio {
		attribute total : Integer;
		attribute parts : Integer;
		require constraint {
			total / parts >= 2
		}
		require constraint {
			parts >= 1
		}
		require constraint {
			parts <= 4
		}
		require constraint {
			total <= 40
		}
		objective mostParts : MaximizeObjective {
			attribute :>> best = parts;
		}
	}

	// A nonlinear objective in a case whose conditions are already nonlinear
	// through a guarded division, which says nothing about the objective.
	analysis def GuardedNonlinearGain {
		attribute total : Integer;
		attribute parts : Integer;
		require constraint {
			total / parts >= 2
		}
		require constraint {
			parts >= 1 and parts <= 4
		}
		objective bestGain : MaximizeObjective {
			attribute :>> best = total * parts;
		}
	}

	// An objective whose value is a product of two computed values, which an
	// optimizer does not improve.
	analysis def NonlinearGain {
		attribute a : Real;
		attribute b : Real;
		require constraint {
			a >= 1.0 and a <= 4.0
		}
		require constraint {
			b >= 1.0 and b <= 4.0
		}
		objective bestGain : MaximizeObjective {
			attribute :>> best = a * b;
		}
	}

	// An objective typed by a requirement definition that says nothing about which
	// way its value is to be improved.
	requirement def FitsWell :> TradeStudyObjective;

	analysis def UndirectedGoal {
		attribute size : Integer;
		require constraint {
			size >= 1
		}
		objective goal : FitsWell {
			attribute :>> best = size;
		}
	}

	// An objective stating no value to improve.
	analysis def ValuelessGoal {
		attribute size : Integer;
		require constraint {
			size >= 1
		}
		objective goal : MinimizeObjective;
	}

	// A case stating conditions but no objective, which is a satisfiability
	// question rather than an optimization.
	analysis def NoGoal {
		attribute size : Integer;
		require constraint {
			size >= 1
		}
	}

	// A model's own objective definition, whose condition bounds the values its
	// objectives improve as an objective's own condition does.
	requirement def LeastMass :> MinimizeObjective {
		attribute :>> best : Integer;
		require constraint {
			best >= 2
		}
	}

	analysis def BoundedByItsDefinition {
		attribute mass : Integer;
		require constraint {
			mass <= 9
		}
		objective lightest : LeastMass {
			attribute :>> best = mass;
		}
	}
}
