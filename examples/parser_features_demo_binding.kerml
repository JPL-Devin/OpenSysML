/*
 * Parser Features Demo - Binding Usage
 * 
 * Demonstrates binding usage special syntax patterns:
 * - Basic binding with multiplicity
 * - Binding with name[mult] pattern
 * - Binding with source expression
 * - Binding with 'of' keyword
 */

package demo_binding {
	
	// Basic binding patterns
	feature basicBindings {
		doc /* Simple binding declarations */
		
		feature targetValue[1];
		feature featureA[1];
		feature featureB[1];
		
		// Multiplicity before name
		binding [1] sourceToTarget = targetValue;
		
		// Named binding
		binding linkAToB of [1] featureA = [1] featureB;
	}
	
	// Advanced binding: name[mult] with source (from ShapeItems.sysml)
	feature advancedBindings {
		doc /* 
		 * Pattern: binding [outer_mult] name[inner_mult] source = value
		 * This pattern was a key fix in Task 54.
		 */
		
		feature base {
			feature edges[0..*];
		}
		feature boundaryEdges[0..*];
		feature system {
			feature outputs[0..*];
		}
		feature monitor {
			feature inputs[0..*];
		}
		
		binding [1] bind [0..*] base.edges = [0..*] boundaryEdges;
		
		// Feature chain as source
		binding [1] dataLink [0..*] system.outputs = [0..*] monitor.inputs;
	}
	
	// Binding with feature chains
	feature chainBindings {
		feature startEvent {
			feature timestamp[1];
		}
		feature endEvent {
			feature timestamp[1];
		}
		feature config {
			feature host[1];
			feature 'port'[1];
		}
		feature serverAddress[1];
		feature serverPort[1];
		
		binding startBinding = startEvent.timestamp;
		binding endBinding = endEvent.timestamp;
		
		// Multiple bindings in sequence
		binding [1] config.host = serverAddress;
		binding [1] config.'port' = serverPort;
	}
	
	// Real-world pattern from stdlib
	item part Shape {
		feature base {
			feature edges[0..*];
		}
		
		feature boundaryEdges[0..*];
		
		// Bind collection elements
		binding [1] bind [0..*] base.edges = [0..*] boundaryEdges {
			doc /* Binds each edge in base to boundary */
		}
	}
}
