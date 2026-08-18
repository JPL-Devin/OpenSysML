// A behaviour an object performs runs as that object, however deeply nested: the
// accept two performances down takes the message addressed to the part by name.
package SendIdentityPerformed {
	action def Inner {
		first start;

		action reader accept n : Integer;

		done fin;

		then start reader;
		then reader fin;
	}

	action def Outer {
		first start;

		action mid : Inner;

		done fin;

		then start mid;
		then mid fin;
	}

	part def Node {
		state Listen {
			initial start;

			state waiting {
				entry send 7 to worker;
			}
			state heard;

			start then waiting;

			transition first waiting then heard do perform Outer;
		}
	}

	part worker : Node;
}
