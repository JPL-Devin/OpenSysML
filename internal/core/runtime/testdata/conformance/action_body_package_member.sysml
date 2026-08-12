// An action body resolves a name against the scope it was written in, so it
// reaches the members of its own package: `pkgLevel` is declared beside the
// action, not in it.
package test {
	public import ScalarValues::*;

	attribute pkgLevel : Real = 7.0;

	action byPkgRef {
		attribute r : Real = 0.0;

		first start;

		action bump {
			assign r := pkgLevel + 1.0;
		}

		done end;

		then start bump;
		then bump end;
	}
}
