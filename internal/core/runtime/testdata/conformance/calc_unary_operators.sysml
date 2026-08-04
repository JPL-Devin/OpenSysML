package test {
    // Test unary operators: negation and logical not
    calc testNegation {
        in x : Integer;
        return -x;
    }
    
    calc testLogicalNot {
        in flag : Boolean;
        return not flag;
    }
}
