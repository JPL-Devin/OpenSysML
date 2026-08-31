library package DocumentQueries {
	/* NON-NORMATIVE OpenSysML document-query vocabulary;
	 * not part of the OMG SysML or KerML standard library. */

	private import KerML::Root::Element;
	private import ScalarValues::*;

	abstract calc def Query {
		return result : Element[0..*] ordered;
	}

	calc def OwnedElements {
		in source : Element[0..*] ordered;
		return result : Element[0..*] ordered;
	}

	calc def Descendants {
		in source : Element[0..*] ordered;
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	calc def Ancestors {
		in source : Element[0..*] ordered;
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	calc def RelatedElements {
		in source : Element[0..*] ordered;
		/* relationshipKind: "specialization", "subsetting", "redefinition",
		 * "typing", "connection", "allocation", "satisfaction" or
		 * "verification"; direction: "outgoing" or "incoming". */
		in relationshipKind : String[1];
		in direction : String[1];
		in maxDepth : Integer[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereType {
		in source : Element[0..*] ordered;
		in type : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereMetadata {
		in source : Element[0..*] ordered;
		in 'metadata' : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereName {
		in source : Element[0..*] ordered;
		in operator : String[1];
		in value : String[1];
		return result : Element[0..*] ordered;
	}

	calc def WhereFeature {
		in source : Element[0..*] ordered;
		in 'feature' : String[1];
		in operator : String[1];
		in value : String[1];
		return result : Element[0..*] ordered;
	}

	calc def OrderBy {
		in source : Element[0..*] ordered;
		in property : String[1];
		in direction : String[1];
		in missing : String[1];
		in multiple : String[1];
		return result : Element[0..*] ordered;
	}

	/* One computed table column: an explicit name plus a calc expression
	 * evaluated on each projected row element. */
	attribute def ColumnSpec;

	/* A column expression references row properties by their declared
	 * feature (e.g. Subsystem::mass), supports +, -, *, / arithmetic,
	 * string concatenation with +, and ?? for absent-value defaults.
	 * A row not conforming to the feature's declaring type reads it as
	 * absent; a failing expression, an absent result not defaulted
	 * with ??, or a multi-valued result fails the query with a typed
	 * error naming the row and column. */
	calc def Column {
		in name : String[1];
		in expression : ScalarValue[0..1];
		return result : ColumnSpec[1];
	}

	calc def Project {
		in source : Element[0..*] ordered;
		in properties : String[0..*] ordered = null;
		in columns : ColumnSpec[0..*] ordered = null;
		return result : Element[0..*] ordered;
	}

	abstract part def Document {
		attribute title : String[1];
	}

	abstract part def Section {
		attribute title : String[1];
	}

	abstract part def ContentBlock;

	part def Paragraph :> ContentBlock {
		attribute text : String[0..1];
	}

	/* Inline runs compose a paragraph in declaration order, joined by
	 * single spaces. */
	abstract part def Run;

	part def Span :> Run {
		attribute text : String[1];
		/* style: "plain", "emphasis", "strong" or "code". */
		attribute style : String[0..1];
	}

	part def Link :> Run {
		attribute text : String[1];
		attribute target : String[1];
	}

	/* Ref links to another named content block of the same document; text
	 * defaults to the target's title, caption or name. */
	part def Ref :> Run {
		attribute text : String[0..1];
		ref target : Element[1];
	}

	part def Table :> ContentBlock {
		attribute caption : String[0..1];
		/* groupBy: a projected column name; rows are grouped by its value
		 * into subtables in order of first appearance. */
		attribute groupBy : String[0..1];
	}

	part def List :> ContentBlock {
		attribute style : String[0..1];
	}

	part def Diagram :> ContentBlock {
		attribute caption : String[0..1];
		/* kind: a supported rendering kind ("tree", "interconnection",
		 * "state", "action", "table" or "sequence"); required when source
		 * is a plain element, stated by the view when source is a view. */
		attribute kind : String[0..1];
		/* direction: "TB", "LR", "RL" or "BT", for kinds drawn as graphs. */
		attribute direction : String[0..1];
		ref source : Element[0..1];
	}
}
