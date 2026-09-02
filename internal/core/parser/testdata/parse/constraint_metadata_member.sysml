package P {
	metadata def Tag { attribute id : String; }
	constraint def Positive {
		@Tag { id = "a"; }
		in x : Real;
		x > 0
	}
	constraint c : Positive {
		@Tag;
	}
	constraint tagged { @Tag }
	constraint tagged2 { @Tag and true }
	constraint tagged3 { @ Tag::sub }
}
