/*
 * Parser Features Demo - Connectors & Succession
 * 
 * Demonstrates connector patterns and succession:
 * - Connection with connect keyword
 * - Succession with first/then keywords
 * - Named vs anonymous succession
 */

package demo_connectors {
	
	// The library is not implicitly visible: a bare name needs an import.
	private import Occurrences::MatesWith;
	
	// The types the connections below use.
	connection def MatingConnection;
	connection def DataLink;
	connection def DataConnection;
	
	// Connection patterns with connect keyword (Task 55 fix)
	feature connectionPatterns {
		doc /* Connection with typing and connect keyword */
		
		feature partA[1];
		feature partB[1];
		feature sensor[1];
		feature processor[1];
		feature actuator[1];
		
		// Named connection with typing
		connection mating :MatingConnection {
			end source[1];
			end target[1];
		}
		
		// Connection with connect keyword (from ShapeItems.sysml pattern)
		connection :MatesWith connect [1] partA to [1] partB;
		
		// Multiple connections
		connection link1 :DataLink connect [1] sensor to [1] processor;
		connection link2 :DataLink connect [1] processor to [1] actuator;
	}
	
	// Succession patterns (Task 56 - major fix)
	feature successionPatterns {
		doc /* 
		 * Succession establishes temporal ordering.
		 * Supports both named and anonymous forms with first/then.
		 */
		
		feature startEvent[1];
		feature endEvent[1];
		feature causes {
			feature startShot[1];
		}
		feature effects[1];
		feature initStep[1];
		feature processStep[1];
		feature input[1];
		feature output[1];
		feature a[1];
		feature b[1];
		
		// Named succession with first/then
		succession eventOrder first [1] startEvent then [1] endEvent;
		
		// Named succession with multiplicities
		succession causalOrdering first [1] causes.startShot then [1] effects {
			doc /* Based on CausationConnections.sysml pattern */
		}
		
		// Anonymous succession - no name, direct to connector ends
		succession [1] initStep then [1] processStep;
		
		// Anonymous with first keyword
		succession first [1] input then [1] output;
		
		// Simple named succession (no first keyword)
		succession sequentialFlow a then b;
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
		succession init first [1] initialize then [1] validate;
		succession validation first [1] validate then [1] execute;
		succession exec first [1] execute then [1] finalize;
		
		// Anonymous succession for conditional paths
		succession [0..1] validate then [0..1] fallback;
	}
	
	// Complex connector scenario
	feature sensorNetwork {
		feature sensors[*] {
			feature measure[1];
		}
		feature hub[1] {
			feature aggregate[1];
		}
		feature processors[*] {
			feature analyze[1];
		}
		
		// Multiple sensors connected to hub
		connection :DataConnection connect [*] sensors to [1] hub;
		
		// Hub connected to multiple processors  
		connection :DataConnection connect [1] hub to [*] processors;
		
		// Succession establishes data flow timing
		succession dataAcquisition 
			first [*] sensors.measure 
			then [1] hub.aggregate;
			
		succession dataProcessing
			first [1] hub.aggregate
			then [*] processors.analyze;
	}
}
