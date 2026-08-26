package Test {
    state Machine {
        entry; then init;
        state init;
        state waiting {
            // Sourceless accept-then transition: inside waiting, accept 'go' and transition to active
            accept go then active;
        }
        state active;
        final done;
        
        succession first init then waiting;
        succession first active then done;
    }
}
