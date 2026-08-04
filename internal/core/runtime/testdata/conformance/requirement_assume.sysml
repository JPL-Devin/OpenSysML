package test {
	requirement SafeOperation {
		assume 1 > 100;  // false assumption (trusted, doesn't fail)
		require 50 < 100;  // true requirement
	}
}
