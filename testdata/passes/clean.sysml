package Lib {
	public namespace Widgets;
	namespace Gadgets;
}

package App {
	import Lib::*;
	alias W for Lib::Widgets;
	dependency from App to Lib;
}
