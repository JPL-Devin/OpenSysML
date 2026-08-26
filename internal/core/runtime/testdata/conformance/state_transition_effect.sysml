package test {
    // Test transition effect actions
    state TestEffects {
        attribute counter : Integer = 0;
        
        entry; then init;
        state init;
        state active;
        final done;
        
        succession first init then active;
        transition first active do {
            counter = counter + 1;
        } then done;
    }
}
