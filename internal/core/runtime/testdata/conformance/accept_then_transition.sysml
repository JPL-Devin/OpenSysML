package Test {
    state Machine {
        initial init;
        state waiting {
            // Sourceless accept-then transition: inside waiting, accept 'go' and transition to active
            accept go then active;
        }
        state active;
        final done;
        
        init then waiting;
        active then done;
    }
}
