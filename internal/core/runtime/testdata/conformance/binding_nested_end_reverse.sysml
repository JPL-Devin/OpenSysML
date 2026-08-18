package test {
	part def Child {
		attribute b = 9;
	}
	part def Sys {
		attribute x;
		part child : Child;
		attribute mirror = child.b;
		binding childBinding bind child.b = x;
	}
}
