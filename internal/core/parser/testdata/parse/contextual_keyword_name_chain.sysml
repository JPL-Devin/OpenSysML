// `chain` is the feature chain modifier only when a name follows it
// (`attribute chain x : T;`); before `=`, `:`, `;`, `[` or a keyword that
// continues the declaration (`default`, `defined by`, `subsets`, a metadata
// usage's `about`) it names the feature, on either side of the kind keyword.
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

	part keywordTail {
		attribute chain default 1;
		attribute chain defined by Integer;
		attribute chain subsets pt;
	}

	metadata def Tag;
	metadata chain : Tag about named;
	metadata chain about bare;

	part modifier {
		attribute chain x : Link;
		ref chain y : Link;
		ref chain 'quoted name' : Link;
	}
}
