package Kit {
	part def Widget;
}

package ErrorViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	// A view exposing nothing: a plain import exposes nothing.
	view emptyView {
		private import Kit::*;
	}

	// A rendering kind Systemica does not produce.
	view tabularView {
		expose Kit::Widget;
		render asElementTable;
	}

	// A sequence view: the notation is understood, the rendering is not produced.
	view sequenceView : SequenceView {
		expose Kit::Widget;
	}

	// A structural element an action rendering cannot show.
	view misfitView : ActionFlowView {
		expose Kit::Widget;
	}
}
