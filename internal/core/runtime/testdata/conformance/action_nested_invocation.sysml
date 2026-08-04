package test {
    // Test nested action invocation
    action outer {
        attribute result : Integer = 0;
        
        first start;
        
        action inner {
            assign result := result + 5;
        }
        
        done end;
        
        then start inner;
        then inner end;
    }
}
