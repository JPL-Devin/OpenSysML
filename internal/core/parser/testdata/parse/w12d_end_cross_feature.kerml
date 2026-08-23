// An association end may declare its cross feature inline, ahead of its own
// declaration (KerML.xtext OwnedCrossFeatureMember).
package W12DEndCrossFeature {
	class C1 {
		feature a : C2;
	}

	class C2 {
		feature b : C1;
	}

	assoc A1 {
		end x1 [0..1] feature x : C1 crosses y.b {
			public import y::y1;
		}
		end feature y : C2 crosses x.y1 {
			member feature y1 [0..1] featured by C1;
		}
	}

	assoc A2 {
		end xa subsets C1::a feature x : C1;
		end feature y : C2;
	}
}
