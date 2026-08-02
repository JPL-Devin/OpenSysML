/*
 * Parser Features Demo - Relationships
 * 
 * Demonstrates new relationship keywords added to the parser:
 * - inverse of: bidirectional relationship inverses
 * - unions: union relationships
 * - chains: feature chaining
 */

package demo_relationships {
	
	// Inverse of relationship - establishes bidirectional links
	assoc struct BidirectionalLink {
		doc /* Example of inverse relationships from Occurrences.kerml */
		
		feature parent: Node[1];
		feature child: Node[0..*] inverse of parent;
	}
	
	// Unions relationship - combines multiple features
	feature unifiedCollection: Item[*] unions setA, setB, setC {
		doc /* Unions combines multiple collections into one */
		
		feature setA: Item[*];
		feature setB: Item[*];
		feature setC: Item[*];
	}
	
	// Chains relationship - feature chaining for navigation
	feature self: Anything[1] subsets things chains things.that {
		doc /* 
		 * Chains enables feature navigation through multiple levels.
		 * Based on Base.kerml pattern.
		 */
	}
	
	feature dataPath: String[1] {
		feature source: DataSource[1];
		feature endpoint: Endpoint[1] chains source.target;
		
		doc /* Chains through source to reach target endpoint */
	}
}
