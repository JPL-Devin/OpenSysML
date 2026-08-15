/*
 * Parser Features Demo - Visibility & Modifiers
 * 
 * Demonstrates visibility modifiers and feature modifiers:
 * - protected/public/private visibility
 * - readonly modifier
 * - constant modifier
 * - nonunique/ordered post-multiplicity modifiers
 */

package demo_modifiers {
	// The library is not implicitly visible: a bare name needs an import.
	private import ScalarValues::*;
	private import Items::Item;

	// The types the demos below refer to.
	classifier Config;
	port def ConnectionPort;

	feature VisibilityExample {
		doc /*
		 * Demo: Visibility and modifier features
		 * 
		 * Access control through visibility modifiers
		 */
		
		public feature publicData: Integer[1];
		protected feature protectedData: Integer[1];
		private feature privateData: Integer[1];
	}
	
	feature readonlyConfig: Config[1] {
		doc /* Readonly modifier creates immutable features */
		
		readonly feature systemName: String[1];
		readonly feature version: String[1];
	}
	
	feature constants {
		doc /* Constant modifier for compile-time constants */
		
		abstract constant ref maxConnections: Integer[1];
		abstract constant ref timeout: Real[1];
		abstract constant ref defaultPort: Integer[1];
	}
	
	feature collections {
		doc /* Ordered and nonunique control collection behavior */
		
		feature orderedList: Item[*] ordered {
			doc /* Ordered collection maintains insertion order */
		}
		
		feature bagWithDuplicates: Item[*] nonunique {
			doc /* Nonunique allows duplicate elements */
		}
		
		feature orderedBag: Item[*] ordered nonunique {
			doc /* Combined: ordered bag */
		}
		
		port connections: ConnectionPort[1..*] nonunique {
			doc /* Port with nonunique after multiplicity (from Interfaces.sysml pattern) */
		}
	}
	
	interface ExampleInterface {
		doc /* Protected reference pattern from real stdlib */
		
		protected ref thisParticipant :>> self;
	}
}
