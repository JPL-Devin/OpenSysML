package OrthogonalRegionsDemo {
    private import SI::*;
    
    // Demonstrates concurrent orthogonal regions in state machines
    // Each region executes independently and processes events in parallel
    
    // Example: Traffic Light with Pedestrian Crossing
    state def TrafficLight parallel {
        // Vehicle signal region
        state vehicleSignal {
            entry; then red;
            state red;
            state green;
            state yellow;
            
            transition first red accept after 30 [s] then green;
            transition first green accept after 25 [s] then yellow;
            transition first yellow accept after 5 [s] then red;
        }
        
        // Pedestrian signal region (runs concurrently)
        state pedestrianSignal {
            entry; then dontWalk;
            state dontWalk;
            state walk;
            
            transition first dontWalk accept after 45 [s] then walk;
            transition first walk accept after 15 [s] then dontWalk;
        }
        
        // Key concepts:
        // - Both regions start simultaneously (entry→red, entry→dontWalk)
        // - Active configuration: (red, dontWalk) or (green, walk) etc
        // - Events broadcast to both regions independently
        // - Run-to-completion: one transition per region per event
    }
}
