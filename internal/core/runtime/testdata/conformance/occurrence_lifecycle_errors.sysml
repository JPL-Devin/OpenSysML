// What the lifecycle functions refuse, each as a typed error where the value is
// read: `destroy` of an occurrence destroyed already, a feature of a destroyed
// occurrence, `addNewAt` past one beyond the group's end, and `create` of an
// occurrence that began before the call. (`isDuring` of a data value is a type
// error the checker reports before execution; the robustness tests cover it.)
package test {
	private import ScalarValues::*;
	private import OccurrenceFunctions::*;
	private import SequenceFunctions::size;

	part def Widget { attribute n : Integer = 1; }

	part bench {
		part twice : Widget;
		part gone : Widget;
		part kept : Widget;
		part late : Widget;
		part group : Widget[0..*];
		attribute destroyedTwice : Boolean = destroy(twice) === twice and destroy(twice) === twice;
		attribute readAfter : Integer = destroy(gone).n;
		attribute tooFar : Integer = size(addNewAt(group, late, 3));
		attribute keptN : Integer = kept.n;
		attribute begunBefore : Boolean = keptN == 1 and create(kept) === kept;
	}
}
