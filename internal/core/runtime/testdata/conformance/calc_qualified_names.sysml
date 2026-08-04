package test {
    package inner {
        attribute value : Integer = 42;
    }
    
    // Test qualified name resolution
    calc testQualified {
        return test::inner::value;
    }
}
