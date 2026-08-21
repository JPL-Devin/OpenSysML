/*
 * Parser Features Demo - Visibility & Modifiers
 * 
 * Demonstrates visibility modifiers and feature modifiers:
 * - protected/public/private visibility
 * - const modifier
 * - var modifier
 * - nonunique/ordered post-multiplicity modifiers
 */

package demo_modifiers {
	// The library is not implicitly visible: a bare name needs an import.
	private import ScalarValues::*;
	private import Items::Item;

	// The types the demos below refer to.
	struct Config;
	classifier ConnectionPort;

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
		doc /* The const modifier creates immutable features */
		
		const feature systemName: String[1];
		const feature version: String[1];
	}
	
	struct Variables {
		doc /* The var modifier marks features whose value may change */
		
		var feature currentConnections: Integer[1];
		var feature elapsed: Real[1];
	}
	
	struct Constants {
		doc /* Const features act as compile-time constants */
		
		const feature maxConnections: Integer[1];
		const feature timeout: Real[1];
		const feature defaultPort: Integer[1];
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
		
		feature connections: ConnectionPort[1..*] nonunique {
			doc /* Typed feature with nonunique after multiplicity */
		}
	}
	
	struct ExampleInterface {
		doc /* Protected redefinition pattern from real stdlib */
		
		protected feature thisParticipant :>> self;
	}
}
