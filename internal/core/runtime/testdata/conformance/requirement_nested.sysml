package test {
	requirement ParentReq {
		require 10 < 20;
		
		requirement NestedReq {
			require 5 < 10;
		}
	}
}
