package test {
    // Test transition effect actions
    state TestEffects {
        attribute counter : Integer = 0;
        
        initial init;
        state active;
        final done;
        
        succession first init then active;
        transition active to done do {
            counter = counter + 1;
        };
    }
}
