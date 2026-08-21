/*
 * Parser Features Demo - Connectors & Succession
 * 
 * Demonstrates SysML connector patterns and succession:
 * - Connection with connect keyword
 * - Succession with first/then keywords
 * - Named vs anonymous succession
 */

package demo_connectors {
	
	// The library is not implicitly visible: a bare name needs an import.
	private import Occurrences::MatesWith;
	
	// The types the connections below use.
	connection def MatingConnection {
		end source[1];
		end target[1];
	}
	connection def DataLink {
		end source[1];
		end target[1];
	}
	connection def DataConnection {
		end source[1];
		end target[1];
	}
	
	// Connection patterns with connect keyword
	part def ConnectionPatterns {
		doc /* Connection with typing and connect keyword */
		
		part partA[1];
		part partB[1];
		part sensor[1];
		part processor[1];
		part actuator[1];
		
		// Named connection with typing
		connection mating : MatingConnection {
			end :>> source ::> partA;
			end :>> target ::> partB;
		}
		
		// Connection with connect keyword (from ShapeItems.sysml pattern)
		connection : MatesWith connect [1] partA to [1] partB;
		
		// Multiple connections
		connection link1 : DataLink connect [1] sensor to [1] processor;
		connection link2 : DataLink connect [1] processor to [1] actuator;
	}
	
	// Succession patterns
	action def SuccessionPatterns {
		doc /* 
		 * Succession establishes temporal ordering.
		 * Supports both named and anonymous forms with first/then.
		 */
		
		action startEvent[1];
		action endEvent[1];
		action causes {
			action startPoint[1];
		}
		action effects[1];
		action initStep[1];
		action processStep[1];
		action input[1];
		action output[1];
		action a[1];
		action b[1];
		
		// Named succession with first/then
		succession eventOrder first startEvent then endEvent;
		
		// Named succession with feature chain ends
		succession causalOrdering first causes.startPoint then effects {
			doc /* Based on CausationConnections.sysml pattern */
		}
		
		// Anonymous succession - no name, direct to connector ends
		succession first initStep then processStep;
		
		// Anonymous with multiplicities on the ends
		succession first [1] input then [1] output;
		
		// Simple named succession
		succession sequentialFlow first a then b;
	}
	
	// Real-world workflow example
	action Workflow {
		doc /* Demonstrates succession in action workflows */
		
		// Sequential steps
		action initialize;
		action validate;
		action execute;
		action finalize;
		action fallback;
		
		// Succession relationships establish order
		succession init first initialize then validate;
		succession validation first validate then execute;
		succession exec first execute then finalize;
		
		// Anonymous succession for conditional paths
		succession first [0..1] validate then [0..1] fallback;
	}
	
	// Complex connector scenario
	part def SensorNetwork {
		part sensors[*] {
			action measure[1];
		}
		part hub[1] {
			action aggregate[1];
		}
		part processors[*] {
			action analyze[1];
		}
		
		// Multiple sensors connected to hub
		connection : DataConnection connect [*] sensors to [1] hub;
		
		// Hub connected to multiple processors  
		connection : DataConnection connect [1] hub to [*] processors;
		
		// Succession establishes data flow timing
		succession dataAcquisition 
			first sensors.measure 
			then hub.aggregate;
			
		succession dataProcessing
			first hub.aggregate
			then processors.analyze;
	}
}
