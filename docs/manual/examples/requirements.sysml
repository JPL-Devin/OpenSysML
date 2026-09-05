// Requirement identifiers and doc text as document prose: the model gives each
// requirement a short name and a doc comment; the Reqs query projects both,
// and the report renders them once as a table and once as definitions.
package Requirements {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;

	requirement def <'HLR-R001'> CrewSafety {
		doc /* The mission shall safely return all three crew members to Earth. */
	}
	requirement def <'HLR-R002'> SoftLanding {
		doc /* The mission shall achieve a soft landing
		     * on the lunar surface. */
	}
	part spec {
		requirement r1 : CrewSafety;
		requirement r2 : SoftLanding;
	}

	calc def Reqs :> Query {
		in root : Element;
		Project(
			source = RelatedElements(
				source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "RequirementUsage"),
				relationshipKind = "typing", direction = "outgoing", maxDepth = 1
			),
			properties = ("shortName", "name", "documentation")
		)
	}

	part def RequirementsReport :> Document {
		attribute redefines title = "Mission Requirements";

		part intro : Paragraph {
			attribute redefines text = "Each requirement by identifier, first as a table and then as prose.";
		}

		part table : Table {
			attribute redefines caption = "Requirements";
			calc rows : Reqs {
				in root = spec;
			}
		}

		part prose : Definitions {
			attribute redefines term = "shortName";
			attribute redefines description = "documentation";
			calc rows : Reqs {
				in root = spec;
			}
		}
	}
}
