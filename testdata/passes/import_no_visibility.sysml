// Deliberately non-conforming notation: the pilot grammar requires a visibility
// indicator before `import`, so each bare import here must warn (and only warn).
package Lib {
	part def Widget;
}

package App {
	import Lib::*;
	part w : Widget;

	part def Holder {
		import Lib::Widget;
	}
}
