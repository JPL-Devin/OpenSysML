package test {
    // Test state do behavior
    state TestDo {
        attribute result : Integer = 0;
        
        initial init;
        state processing {
            do {
                result = result + 10;
            }
        }
        final done;
        
        init then processing;
        processing then done;
    }
}
