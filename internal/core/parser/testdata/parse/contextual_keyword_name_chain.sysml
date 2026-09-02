// `chain` is the feature chain modifier only when a name follows it
// (`attribute chain x : T;`); before `=`, `:`, `;` or `[` it names the feature,
// on either side of the kind keyword.
package P {
	part def Link {
		attribute chain : Integer;
		attribute next = chain + 1;
	}

	part named {
		attribute chain = 1;
		attribute pt = chain + 1;
	}

	part bare {
		attribute chain;
		attribute chain[*] :> pt;
	}

	part modifier {
		attribute chain x : Link;
		ref chain y : Link;
		ref chain 'quoted name' : Link;
	}
}
